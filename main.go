package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Debounce struct {
	CooldownPeriod time.Duration `json:"cooldown_period"`
	Command        []string      `json:"command"`
	hash           string        `json:"-"`
}

var ConfigPath = filepath.Join(Must(os.UserHomeDir()), ".config", "debounce")

func Must[T any](result T, err error) T {
	if err != nil {
		panic(err)
	}
	return result
}

func NewDebounce(args []string) (*Debounce, error) {
	var err error
	if len(args) < 3 {
		return nil, errors.New("not enough arguments (first provide the cooldown value, followed by the command and its arguments)")
	}
	result := Debounce{}
	if duration, err := time.ParseDuration(args[1]); err == nil {
		result.CooldownPeriod = duration
	} else {
		return nil, err
	}
	result.Command = args[2:]
	result.hash, err = calculateHash(result.Command)
	return &result, err
}

func (d *Debounce) isRunnable() bool {
	return time.Since(d.readHash()) > d.CooldownPeriod
}

func Cleanup() {
	entries, err := os.ReadDir(ConfigPath)
	if err != nil {
		log("Failed to read directory %v: %v", ConfigPath, err)
		return
	}

	if len(entries) < 20 {
		// there's no point in cleaning up with so few entries
		return
	}

	log("cleaning up expired cooldown markers")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			log("cannot read info for %v: %v", entry.Name(), err)
			continue
		}
		fullPath := filepath.Join(ConfigPath, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			log("cannot read content of %v: %v", entry.Name(), err)
			continue
		}
		var debounce Debounce
		if err := json.Unmarshal(content, &debounce); err != nil {
			log("cannot understand the content of %v: %v", entry.Name(), err)
			continue
		}
		// log("%v %v %v %v", fullPath, info.ModTime(), time.Since(info.ModTime()), debounce.CooldownPeriod)
		if time.Since(info.ModTime()) > debounce.CooldownPeriod {
			if err := os.Remove(fullPath); err != nil {
				log("cannot delete %v: %v", entry.Name(), err)
				continue
			}
		}
	}
}

func calculateHash(args []string) (string, error) {
	hasher := sha256.New()
	cmd := args[0]
	resolvedPath, err := filepath.EvalSymlinks(cmd)
	if err == nil {
		cmd = resolvedPath
	}
	cmd, err = exec.LookPath(cmd)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(cmd)
	if err == nil {
		hasher.Write([]byte(absolutePath))
	} else {
		return "", fmt.Errorf("%q could not be resolved to an absolute path", args[0])
	}
	for _, arg := range args[1:] {
		hasher.Write([]byte(arg))
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (d *Debounce) readHash() time.Time {
	info, err := os.Stat(filepath.Join(ConfigPath, d.hash))
	if err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func (d *Debounce) writeHash() {
	if err := os.MkdirAll(ConfigPath, 0700); err != nil {
		log("debounce could not access %v", ConfigPath)
	}
	content, err := json.Marshal(d)
	if err != nil {
		log("debounce could not serialise %+v", *d)
	}
	if err := os.WriteFile(filepath.Join(ConfigPath, d.hash), content, 0600); err != nil {
		log("debounce could not write to %v", ConfigPath)
	}
}

func (d *Debounce) Run() int {
	cmd := exec.Command(d.Command[0], d.Command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		d.writeHash()
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	log("%v", err)
	return 1
}

func main() {
	if debounce, err := NewDebounce(os.Args); err == nil {
		if debounce.isRunnable() {
			// 1% of the time, when actually running a command, first cleanup
			// any expired markers
			if rand.Float32() < 0.01 {
				Cleanup()
			}
			os.Exit(debounce.Run())
		}
	} else {
		log("debounce: %v", err)
		os.Exit(1)
	}
}

func log(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("%v\n", msg), args...)
}
