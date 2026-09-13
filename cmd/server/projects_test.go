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
