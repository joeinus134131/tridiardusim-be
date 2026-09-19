package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Component struct {
	ID       string                 `json:"id"`
	TypeID   string                 `json:"typeId"`
	Name     string                 `json:"name"`
	Position []float64              `json:"position"`
	Rotation []float64              `json:"rotation"`
	State    map[string]interface{} `json:"state"`
}

type Wire struct {
	Path              [][]float64 `json:"path,omitempty"`
	ID                string      `json:"id"`
	SourceComponentID string      `json:"sourceComponentId"`
	SourcePinID       string      `json:"sourcePinId"`
	TargetComponentID string      `json:"targetComponentId"`
	TargetPinID       string      `json:"targetPinId"`
	Color             string      `json:"color"`
}

type Project struct {
	Version    int         `json:"version"`
	Name       string      `json:"name"`
	Code       string      `json:"code"`
	Components []Component `json:"components"`
	Wires      []Wire      `json:"wires"`
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var hexColor = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)

func PinValid(kind, pin string) bool {
	switch kind {
	case "esp32_wroom":
		for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 21, 22, 23, 25, 26, 27, 32, 33, 34, 35, 36, 39} {
			if pin == fmt.Sprint("GPIO", n) {
				return true
			}
		}
		for _, p := range []string{"EN", "3V3", "5V", "GND1", "GND2", "GND3"} {
			if p == pin {
				return true
			}
		}
	case "arduino_uno":
		for i := 0; i < 14; i++ {
			if pin == fmt.Sprint("D", i) {
				return true
			}
		}
		for i := 0; i < 6; i++ {
			if pin == fmt.Sprint("A", i) {
				return true
			}
		}
		for _, p := range []string{"5V", "3V3", "GND1", "GND2", "GND3", "RESET", "IOREF", "NC", "VIN", "AREF", "SDA", "SCL", "ICSP_5V", "ICSP_GND", "ICSP_RESET", "ICSP_MOSI", "ICSP_MISO", "ICSP_SCK", "USB_ICSP_MISO", "USB_ICSP_5V", "USB_ICSP_SCK", "USB_ICSP_MOSI", "USB_ICSP_RESET", "USB_ICSP_GND"} {
			if pin == p {
				return true
			}
		}
	case "breadboard":
		for c := 0; c < 30; c++ {
			for r := 0; r < 5; r++ {
				if pin == fmt.Sprintf("t%d_%d", c, r) || pin == fmt.Sprintf("b%d_%d", c, r) {
					return true
				}
			}
			for _, rail := range []string{"pt1", "pt2", "pb1", "pb2"} {
				if c < 25 && pin == fmt.Sprintf("%s_%d", rail, c) {
					return true
				}
			}
		}
	case "led_red", "capacitor_universal":
		return pin == "A" || pin == "C" || pin == "1" || pin == "2"
	case "push_button":
		return pin == "1a" || pin == "1b" || pin == "2a" || pin == "2b"
	case "potentiometer":
		return pin == "1" || pin == "2" || pin == "W"
	case "resistor_220", "jumper_red", "jumper_black", "jumper_blue", "jumper_green", "jumper_yellow":
		return pin == "L" || pin == "R"
	}
	return false
}

func ValidateProject(p Project) error {
	if p.Version != 1 || len(p.Name) == 0 || len(p.Name) > 128 || len(p.Code) > 64000 || len(p.Components) > 100 || len(p.Wires) > 500 || p.Components == nil || p.Wires == nil {
		return fmt.Errorf("invalid project schema/limits")
	}
	ids := map[string]string{}
	for _, c := range p.Components {
		if !safeID.MatchString(c.ID) || ids[c.ID] != "" || len(c.Name) == 0 || len(c.Name) > 128 || len(c.Position) != 3 || len(c.Rotation) != 3 || c.State == nil {
			return fmt.Errorf("invalid component")
		}
		known := false
		for _, test := range []string{"GPIO0", "D0", "t0_0", "A", "1a", "W", "L"} {
			if PinValid(c.TypeID, test) {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("unknown component")
		}
		for _, v := range append(c.Position, c.Rotation...) {
			if v > 10000 || v < -10000 {
				return fmt.Errorf("invalid transform")
			}
		}
		ids[c.ID] = c.TypeID
	}
	seen := map[string]bool{}
	wireIDs := map[string]bool{}
	for _, w := range p.Wires {
		a, b := w.SourceComponentID+":"+w.SourcePinID, w.TargetComponentID+":"+w.TargetPinID
		if !safeID.MatchString(w.ID) || wireIDs[w.ID] || !hexColor.MatchString(w.Color) || a == b || !PinValid(ids[w.SourceComponentID], w.SourcePinID) || !PinValid(ids[w.TargetComponentID], w.TargetPinID) || seen[a+"/"+b] || seen[b+"/"+a] {
			return fmt.Errorf("invalid wire")
		}
		if len(w.Path) > 12 {
			return fmt.Errorf("too many routing points")
		}
		for _, point := range w.Path {
			if len(point) != 3 {
				return fmt.Errorf("invalid routing point")
			}
			for _, v := range point {
				if v > 10000 || v < -10000 {
					return fmt.Errorf("invalid routing coordinate")
				}
			}
		}
		seen[a+"/"+b] = true
		wireIDs[w.ID] = true
	}
	return nil
}

func RegisterProjects(app *fiber.App, dir string) {
	var mu sync.Mutex
	app.Get("/api/capabilities", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"projectVersion": 1, "execution": "browser-worker-subset", "physics": "quasi-static-dc", "maxComponents": 100, "maxWires": 500})
	})
	app.Get("/api/projects", func(c *fiber.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return fiber.ErrInternalServerError
		}
		list := []fiber.Map{}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var p Project
			if json.Unmarshal(data, &p) == nil {
				list = append(list, fiber.Map{"id": e.Name()[:len(e.Name())-5], "name": p.Name})
			}
		}
		return c.JSON(fiber.Map{"projects": list})
	})
	app.Post("/api/projects", func(c *fiber.Ctx) error {
		var p Project
		if err := c.BodyParser(&p); err != nil {
			return fiber.NewError(400, "invalid JSON")
		}
		if err := ValidateProject(p); err != nil {
			return fiber.NewError(400, err.Error())
		}
		mu.Lock()
		defer mu.Unlock()
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fiber.ErrInternalServerError
		}
		id := uuid.NewString()
		data, err := json.Marshal(p)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		tmp, err := os.CreateTemp(dir, ".project-*")
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer os.Remove(tmp.Name())
		if _, err = tmp.Write(data); err != nil {
			tmp.Close()
			return fiber.ErrInternalServerError
		}
		if err = tmp.Close(); err != nil {
			return fiber.ErrInternalServerError
		}
		if err = os.Rename(tmp.Name(), filepath.Join(dir, id+".json")); err != nil {
			return fiber.ErrInternalServerError
		}
		return c.Status(201).JSON(fiber.Map{"id": id, "name": p.Name})
	})
	app.Get("/api/projects/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return fiber.ErrBadRequest
		}
		mu.Lock()
		defer mu.Unlock()
		data, err := os.ReadFile(filepath.Join(dir, id+".json"))
		if os.IsNotExist(err) {
			return fiber.ErrNotFound
		}
		if err != nil {
			return fiber.ErrInternalServerError
		}
		c.Type("json")
		return c.Send(data)
	})
}
