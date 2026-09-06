package main

import (
	"fmt"
	"testing"
)

type fakeDeviceLister struct {
	devs []usbCameraDevice
	err  error
}

func (f *fakeDeviceLister) listDevices() ([]usbCameraDevice, error) {
	return f.devs, f.err
}

func TestResolveUSBCamera_EmptyName(t *testing.T) {
	_, err := resolveUSBCamera("", []usbCameraDevice{{Index: 0, Name: "Cam"}})
	if err == nil {
		t.Fatal("expected error for empty camera name")
	}
}

func TestResolveUSBCamera_NoDevices(t *testing.T) {
	_, err := resolveUSBCamera("Cam", nil)
	if err == nil {
		t.Fatal("expected error when no devices are present")
	}
}

func TestResolveUSBCamera_ExactMatch(t *testing.T) {
	devs := []usbCameraDevice{
		{Index: 0, Name: "FaceTime HD Camera"},
		{Index: 1, Name: "Logitech C920"},
	}
	idx, err := resolveUSBCamera("Logitech C920", devs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}
}

func TestResolveUSBCamera_CaseInsensitive(t *testing.T) {
	devs := []usbCameraDevice{
		{Index: 0, Name: "FaceTime HD Camera"},
	}
	idx, err := resolveUSBCamera("facetime hd camera", devs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
}

func TestResolveUSBCamera_WhitespaceNormalization(t *testing.T) {
	devs := []usbCameraDevice{
		{Index: 2, Name: "  Logitech   C920  "},
	}
	idx, err := resolveUSBCamera("Logitech C920", devs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 2 {
		t.Fatalf("expected index 2, got %d", idx)
	}
}

func TestResolveUSBCamera_SubstringMatch(t *testing.T) {
	devs := []usbCameraDevice{
		{Index: 0, Name: "Built-in FaceTime HD Camera"},
	}
	idx, err := resolveUSBCamera("FaceTime HD", devs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
}

func TestResolveUSBCamera_NoMatch(t *testing.T) {
	devs := []usbCameraDevice{
		{Index: 0, Name: "FaceTime HD Camera"},
	}
	_, err := resolveUSBCamera("Nonexistent Camera", devs)
	if err == nil {
		t.Fatal("expected error when camera name does not match")
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  Logitech   C920  ", "logitech c920"},
		{"FaceTime\tHD\nCamera", "facetime hd camera"},
		{"UPPER CASE", "upper case"},
	}
	for _, c := range cases {
		got := normalizeName(c.in)
		if got != c.want {
			t.Errorf("normalizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveUSBCamera_UsesDeviceLister(t *testing.T) {
	fake := &fakeDeviceLister{
		devs: []usbCameraDevice{
			{Index: 5, Name: "Test Camera"},
		},
	}
	devs, err := fake.listDevices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx, err := resolveUSBCamera("Test Camera", devs)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if idx != 5 {
		t.Fatalf("expected index 5, got %d", idx)
	}
}

func Example_resolveUSBCamera() {
	devs := []usbCameraDevice{
		{Index: 0, Name: "FaceTime HD Camera"},
		{Index: 1, Name: "Logitech C920"},
	}
	idx, err := resolveUSBCamera("logitech c920", devs)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(idx)
	// Output: 1
}
