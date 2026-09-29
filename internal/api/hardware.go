package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type BoardProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	FQBN string `json:"fqbn"`
	Core string `json:"core"`
}

var supportedBoards = map[string]BoardProfile{
	"arduino_uno": {
		ID:   "arduino_uno",
		Name: "Arduino Uno R3",
		FQBN: "arduino:avr:uno",
		Core: "arduino:avr",
	},
	"esp32_wroom": {
		ID:   "esp32_wroom",
		Name: "ESP32-WROOM DevKit",
		FQBN: "esp32:esp32:esp32",
		Core: "esp32:esp32",
	},
}

func findArduinoCLI() string {
	if custom := os.Getenv("ARDUINO_CLI_PATH"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom
		}
	}
	candidates := []string{
		"/opt/homebrew/bin/arduino-cli",
		"/usr/local/bin/arduino-cli",
		"/usr/bin/arduino-cli",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if lp, err := exec.LookPath("arduino-cli"); err == nil {
		return lp
	}
	return ""
}

type CompileRequest struct {
	Code  string `json:"code"`
	Board string `json:"board"`
	FQBN  string `json:"fqbn,omitempty"`
}

type SerialPortInfo struct {
	Address   string `json:"address"`
	Label     string `json:"label"`
	Protocol  string `json:"protocol"`
	BoardName string `json:"boardName,omitempty"`
	FQBN      string `json:"fqbn,omitempty"`
}

var flashRegex = regexp.MustCompile(`Sketch uses (\d+) bytes \((\d+)%\) of program storage space\. Maximum is (\d+) bytes`)
var sramRegex = regexp.MustCompile(`Global variables use (\d+) bytes \((\d+)%\) of dynamic memory`)

func resolveFQBN(board, rawFQBN string) (string, error) {
	if profile, ok := supportedBoards[board]; ok {
		return profile.FQBN, nil
	}
	if rawFQBN != "" {
		if rawFQBN == "arduino:avr:uno" || rawFQBN == "esp32:esp32:esp32" || strings.HasPrefix(rawFQBN, "esp32:esp32:") {
			return rawFQBN, nil
		}
	}
	return "", fmt.Errorf("unsupported board: %s", board)
}

// isAVRBoard returns true if the FQBN is for an AVR-based board (e.g., Arduino Uno).
// AVR boards produce .hex files, while ESP32 boards produce .bin files.
func isAVRBoard(fqbn string) bool {
	return strings.HasPrefix(fqbn, "arduino:avr:")
}

// findCompiledBinary locates the compiled firmware file (.hex for AVR, .bin for ESP32)
// in the output directory after arduino-cli compile --output-dir.
func findCompiledBinary(outputDir string, fqbn string) (string, error) {
	avr := isAVRBoard(fqbn)

	var targetExt string
	if avr {
		// AVR produces sketch.ino.hex (or sketch.ino.with_bootloader.hex)
		targetExt = ".hex"
	} else {
		// ESP32 produces sketch.ino.bin (or sketch.ino.merged.bin)
		targetExt = ".bin"
	}

	var candidates []string
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, targetExt) {
			// For AVR: prefer the one WITHOUT "with_bootloader" suffix
			// For ESP32: prefer the one WITHOUT "merged" or "bootloader" suffix
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("gagal membaca direktori output: %v", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("file firmware %s tidak ditemukan di hasil kompilasi", targetExt)
	}

	// Pick the best candidate
	// For AVR: prefer .hex without "with_bootloader"
	// For ESP32: prefer .bin without "merged" or "bootloader" or "partitions"
	var best string
	for _, c := range candidates {
		base := filepath.Base(c)
		if avr {
			if !strings.Contains(base, "with_bootloader") && !strings.Contains(base, "bootloader") {
				best = c
				break
			}
		} else {
			if !strings.Contains(base, "merged") && !strings.Contains(base, "bootloader") && !strings.Contains(base, "partitions") {
				best = c
				break
			}
		}
	}
	if best == "" {
		// Fallback to first candidate
		best = candidates[0]
	}

	return best, nil
}

func RegisterHardware(app *fiber.App) {
	app.Get("/api/hardware/status", func(c *fiber.Ctx) error {
		cliPath := findArduinoCLI()
		installed := cliPath != ""

		boardsList := []BoardProfile{}
		for _, b := range supportedBoards {
			boardsList = append(boardsList, b)
		}

		return c.JSON(fiber.Map{
			"installed": installed,
			"cliPath":   cliPath,
			"boards":    boardsList,
		})
	})

	// Legacy port scanning endpoint. When backend is deployed to a remote server,
	// this will only detect ports on the SERVER, not on the user's local machine.
	// The frontend should use the Web Serial API (navigator.serial) directly instead.
	app.Get("/api/hardware/ports", func(c *fiber.Ctx) error {
		cliPath := findArduinoCLI()
		ports := []SerialPortInfo{}

		if cliPath != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, cliPath, "board", "list", "--format", "json")
			out, err := cmd.Output()
			if err == nil {
				var data struct {
					DetectedPorts []struct {
						Port struct {
							Address  string `json:"address"`
							Protocol string `json:"protocol"`
							Label    string `json:"label"`
						} `json:"port"`
						MatchingBoards []struct {
							Name string `json:"name"`
							FQBN string `json:"fqbn"`
						} `json:"matching_boards"`
					} `json:"detected_ports"`
				}
				if json.Unmarshal(out, &data) == nil {
					for _, p := range data.DetectedPorts {
						if p.Port.Address == "" {
							continue
						}
						// Filter out bluetooth and virtual debugging devices
						addrLower := strings.ToLower(p.Port.Address)
						if strings.Contains(addrLower, "bluetooth") || strings.Contains(addrLower, "wlan-debug") || strings.Contains(addrLower, "debug-console") {
							continue
						}
						boardName := ""
						fqbn := ""
						if len(p.MatchingBoards) > 0 {
							boardName = p.MatchingBoards[0].Name
							fqbn = p.MatchingBoards[0].FQBN
						}
						label := p.Port.Label
						if label == "" {
							label = filepath.Base(p.Port.Address)
							if boardName != "" {
								label = fmt.Sprintf("%s (%s)", label, boardName)
							}
						}
						ports = append(ports, SerialPortInfo{
							Address:   p.Port.Address,
							Label:     label,
							Protocol:  p.Port.Protocol,
							BoardName: boardName,
							FQBN:      fqbn,
						})
					}
				}
			}
		}

		// Also scan OS device patterns if empty
		if len(ports) == 0 {
			patterns := []string{
				"/dev/cu.usbmodem*",
				"/dev/cu.usbserial*",
				"/dev/cu.SLAB_USBtoUART*",
				"/dev/cu.wchusbserial*",
				"/dev/ttyUSB*",
				"/dev/ttyACM*",
			}
			for _, pat := range patterns {
				matches, _ := filepath.Glob(pat)
				for _, m := range matches {
					ports = append(ports, SerialPortInfo{
						Address:  m,
						Label:    filepath.Base(m),
						Protocol: "serial",
					})
				}
			}
		}

		return c.JSON(fiber.Map{
			"ports": ports,
		})
	})

	// Verify/Compile only endpoint — returns log, memory stats, but NOT the binary.
	// Used for the "Verify" button in the frontend.
	app.Post("/api/hardware/compile", func(c *fiber.Ctx) error {
		cliPath := findArduinoCLI()
		if cliPath == "" {
			return c.Status(503).JSON(fiber.Map{
				"success": false,
				"log":     "arduino-cli tidak terdeteksi pada server/sistem lokal.",
			})
		}

		var req CompileRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": "invalid JSON"})
		}
		if strings.TrimSpace(req.Code) == "" {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": "kode sketch tidak boleh kosong"})
		}

		fqbn, err := resolveFQBN(req.Board, req.FQBN)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": err.Error()})
		}

		tmpDir, err := os.MkdirTemp("", "ardusim-build-*")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal membuat direktori build sementara"})
		}
		defer os.RemoveAll(tmpDir)

		sketchDir := filepath.Join(tmpDir, "sketch")
		if err := os.MkdirAll(sketchDir, 0700); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal membuat direktori sketch"})
		}

		inoPath := filepath.Join(sketchDir, "sketch.ino")
		if err := os.WriteFile(inoPath, []byte(req.Code), 0600); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal menulis file sketch"})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, cliPath, "compile", "--fqbn", fqbn, sketchDir)
		outputBytes, cmdErr := cmd.CombinedOutput()
		logText := string(outputBytes)

		flashBytes := 0
		flashPercent := 0
		flashMax := 0
		sramBytes := 0
		sramPercent := 0

		if fMatch := flashRegex.FindStringSubmatch(logText); len(fMatch) >= 4 {
			flashBytes, _ = strconv.Atoi(fMatch[1])
			flashPercent, _ = strconv.Atoi(fMatch[2])
			flashMax, _ = strconv.Atoi(fMatch[3])
		}
		if sMatch := sramRegex.FindStringSubmatch(logText); len(sMatch) >= 3 {
			sramBytes, _ = strconv.Atoi(sMatch[1])
			sramPercent, _ = strconv.Atoi(sMatch[2])
		}

		if cmdErr != nil {
			return c.JSON(fiber.Map{
				"success": false,
				"log":     logText,
				"fqbn":    fqbn,
			})
		}

		return c.JSON(fiber.Map{
			"success":      true,
			"log":          logText,
			"fqbn":         fqbn,
			"flashBytes":   flashBytes,
			"flashPercent": flashPercent,
			"flashMax":     flashMax,
			"sramBytes":    sramBytes,
			"sramPercent":  sramPercent,
		})
	})

	// Compile & export binary endpoint — compiles the sketch AND returns the compiled
	// firmware binary (base64-encoded) for browser-side flashing via Web Serial.
	// For AVR boards: returns Intel HEX content (text, base64-encoded).
	// For ESP32 boards: returns raw .bin content (binary, base64-encoded).
	app.Post("/api/hardware/compile-binary", func(c *fiber.Ctx) error {
		cliPath := findArduinoCLI()
		if cliPath == "" {
			return c.Status(503).JSON(fiber.Map{
				"success": false,
				"log":     "arduino-cli tidak terdeteksi pada server.",
			})
		}

		var req CompileRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": "invalid JSON"})
		}
		if strings.TrimSpace(req.Code) == "" {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": "kode sketch tidak boleh kosong"})
		}

		fqbn, err := resolveFQBN(req.Board, req.FQBN)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "log": err.Error()})
		}

		tmpDir, err := os.MkdirTemp("", "ardusim-compile-bin-*")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal membuat direktori build"})
		}
		defer os.RemoveAll(tmpDir)

		sketchDir := filepath.Join(tmpDir, "sketch")
		outputDir := filepath.Join(tmpDir, "output")
		if err := os.MkdirAll(sketchDir, 0700); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal membuat direktori sketch"})
		}
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal membuat direktori output"})
		}

		inoPath := filepath.Join(sketchDir, "sketch.ino")
		if err := os.WriteFile(inoPath, []byte(req.Code), 0600); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "log": "gagal menulis file sketch"})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		// Compile with --output-dir to get the binary file in a known location
		cmd := exec.CommandContext(ctx, cliPath, "compile", "--fqbn", fqbn, "--output-dir", outputDir, sketchDir)
		outputBytes, cmdErr := cmd.CombinedOutput()
		logText := string(outputBytes)

		flashBytes := 0
		flashPercent := 0
		flashMax := 0
		sramBytes := 0
		sramPercent := 0

		if fMatch := flashRegex.FindStringSubmatch(logText); len(fMatch) >= 4 {
			flashBytes, _ = strconv.Atoi(fMatch[1])
			flashPercent, _ = strconv.Atoi(fMatch[2])
			flashMax, _ = strconv.Atoi(fMatch[3])
		}
		if sMatch := sramRegex.FindStringSubmatch(logText); len(sMatch) >= 3 {
			sramBytes, _ = strconv.Atoi(sMatch[1])
			sramPercent, _ = strconv.Atoi(sMatch[2])
		}

		if cmdErr != nil {
			return c.JSON(fiber.Map{
				"success": false,
				"log":     logText,
				"fqbn":    fqbn,
			})
		}

		// Find the compiled binary file
		binaryPath, findErr := findCompiledBinary(outputDir, fqbn)
		if findErr != nil {
			return c.JSON(fiber.Map{
				"success": false,
				"log":     fmt.Sprintf("Kompilasi berhasil tetapi file binary tidak ditemukan: %v\n\n%s", findErr, logText),
				"fqbn":    fqbn,
			})
		}

		// Read the binary file
		binaryData, readErr := os.ReadFile(binaryPath)
		if readErr != nil {
			return c.JSON(fiber.Map{
				"success": false,
				"log":     fmt.Sprintf("Gagal membaca file firmware: %v", readErr),
				"fqbn":    fqbn,
			})
		}

		// Encode to base64 for JSON transport
		binaryBase64 := base64.StdEncoding.EncodeToString(binaryData)

		// Determine file format
		fileFormat := "bin"
		if isAVRBoard(fqbn) {
			fileFormat = "hex"
		}

		return c.JSON(fiber.Map{
			"success":      true,
			"log":          logText,
			"fqbn":         fqbn,
			"flashBytes":   flashBytes,
			"flashPercent": flashPercent,
			"flashMax":     flashMax,
			"sramBytes":    sramBytes,
			"sramPercent":  sramPercent,
			"firmware":     binaryBase64,
			"fileFormat":   fileFormat,
			"fileName":     filepath.Base(binaryPath),
		})
	})
}
