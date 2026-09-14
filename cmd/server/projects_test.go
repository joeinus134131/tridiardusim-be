package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectRoundTrip(t *testing.T) {
	app := fiber.New()
	registerProjects(app, t.TempDir())
	body := `{"version":1,"name":"uji","code":"void setup() {} void loop() {}","components":[{"id":"uno","typeId":"arduino_uno","name":"Uno","position":[0,0,0],"rotation":[0,0,0],"state":{}}],"wires":[]}`
	req := httptest.NewRequest("POST", "/api/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("save: %s", b)
	}
	var saved map[string]string
	json.NewDecoder(res.Body).Decode(&saved)
	res, err = app.Test(httptest.NewRequest("GET", "/api/projects/"+saved["id"], nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var p project
	if err = json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "uji" || len(p.Components) != 1 || p.Code != "void setup() {} void loop() {}" {
		t.Fatal("roundtrip lost data")
	}
	res, err = app.Test(httptest.NewRequest("GET", "/api/projects", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var list struct {
		Projects []map[string]string `json:"projects"`
	}
	json.NewDecoder(res.Body).Decode(&list)
	if len(list.Projects) != 1 {
		t.Fatal("missing saved project")
	}
	for _, bad := range []string{`{}`, strings.Replace(body, `"version":1`, `"version":2`, 1), strings.Replace(body, `"arduino_uno"`, `"unknown"`, 1), strings.Replace(body, `"wires":[]`, `"wires":[{"id":"w","sourceComponentId":"uno","sourcePinId":"D99","targetComponentId":"uno","targetPinId":"GND1","color":"#ffffff"}]`, 1)} {
		req = httptest.NewRequest("POST", "/api/projects", strings.NewReader(bad))
		req.Header.Set("Content-Type", "application/json")
		res, err = app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 400 {
			t.Errorf("invalid accepted: %s", bad)
		}
	}
}

func TestESP32RoutingRoundTrip(t *testing.T) {
	app := fiber.New()
	registerProjects(app, t.TempDir())
	body := `{"version":1,"name":"ESP32 route","code":"","components":[{"id":"esp","typeId":"esp32_wroom","name":"ESP32","position":[0,0.6,0],"rotation":[0,0,0],"state":{}}],"wires":[{"id":"w","sourceComponentId":"esp","sourcePinId":"GPIO25","targetComponentId":"esp","targetPinId":"GPIO26","color":"#a855f7","path":[[1,4,2],[3,4,2]]}]}`
	req := httptest.NewRequest("POST", "/api/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatal("ESP32 save rejected")
	}
	var saved map[string]string
	json.NewDecoder(res.Body).Decode(&saved)
	res, err = app.Test(httptest.NewRequest("GET", "/api/projects/"+saved["id"], nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var p project
	json.NewDecoder(res.Body).Decode(&p)
	if len(p.Wires) != 1 || len(p.Wires[0].Path) != 2 || p.Wires[0].Path[0][1] != 4 || p.Wires[0].Color != "#a855f7" {
		t.Fatal("route lost")
	}
	p.Wires[0].Path = [][]float64{{1, 2}}
	if validateProject(p) == nil {
		t.Fatal("invalid point accepted")
	}
}
