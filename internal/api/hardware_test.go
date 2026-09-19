package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHardwareStatus(t *testing.T) {
	app := fiber.New()
	RegisterHardware(app)

	req := httptest.NewRequest("GET", "/api/hardware/status", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var status struct {
		Installed bool           `json:"installed"`
		CLIPath   string         `json:"cliPath"`
		Boards    []BoardProfile `json:"boards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}

	if len(status.Boards) < 2 {
		t.Fatalf("expected at least 2 supported boards, got %d", len(status.Boards))
	}
}

func TestHardwarePorts(t *testing.T) {
	app := fiber.New()
	RegisterHardware(app)

	req := httptest.NewRequest("GET", "/api/hardware/ports", nil)
	res, err := app.Test(req, 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var portsResp struct {
		Ports []SerialPortInfo `json:"ports"`
	}
	if err := json.NewDecoder(res.Body).Decode(&portsResp); err != nil {
		t.Fatal(err)
	}
}

func TestHardwareCompileValidation(t *testing.T) {
	app := fiber.New()
	RegisterHardware(app)

	// Empty code should be rejected
	req := httptest.NewRequest("POST", "/api/hardware/compile", strings.NewReader(`{"code":"","board":"arduino_uno"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("expected 400 for empty code, got %d", res.StatusCode)
	}

	// Invalid board should be rejected
	req = httptest.NewRequest("POST", "/api/hardware/compile", strings.NewReader(`{"code":"void setup(){}","board":"invalid_board"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid board, got %d", res.StatusCode)
	}
}

func TestHardwareCompileLive(t *testing.T) {
	cli := findArduinoCLI()
	if cli == "" {
		t.Skip("arduino-cli not installed, skipping live compile test")
	}

	app := fiber.New()
	RegisterHardware(app)

	body := `{"code":"void setup(){pinMode(13,OUTPUT);}void loop(){digitalWrite(13,HIGH);delay(100);}","board":"arduino_uno"}`
	req := httptest.NewRequest("POST", "/api/hardware/compile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, 60000)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, b)
	}

	var compileResp struct {
		Success      bool   `json:"success"`
		Log          string `json:"log"`
		FlashBytes   int    `json:"flashBytes"`
		FlashPercent int    `json:"flashPercent"`
	}
	if err := json.NewDecoder(res.Body).Decode(&compileResp); err != nil {
		t.Fatal(err)
	}
	if !compileResp.Success {
		t.Fatalf("compilation failed: %s", compileResp.Log)
	}
	if compileResp.FlashBytes <= 0 {
		t.Fatalf("expected flash bytes > 0, got %d", compileResp.FlashBytes)
	}
}
