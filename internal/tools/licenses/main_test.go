package main

import "testing"

func TestDetectLicense(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "Apache", text: "Apache License\nVersion 2.0", want: "Apache-2.0"},
		{name: "MIT", text: "Permission is hereby granted, free of charge", want: "MIT"},
		{name: "ISC", text: "Permission to use, copy, modify, and/or distribute", want: "ISC"},
		{name: "BSD 3 clause", text: "Redistribution and use in source and binary forms\nNeither the name", want: "BSD-3-Clause"},
		{name: "BSD 2 clause", text: "Redistribution and use in source and binary forms", want: "BSD-2-Clause"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := detectLicense(test.text)
			if err != nil {
				t.Fatalf("detectLicense: %v", err)
			}
			if got != test.want {
				t.Fatalf("detectLicense = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDetectLicenseRejectsUnknownText(t *testing.T) {
	if _, err := detectLicense("proprietary"); err == nil {
		t.Fatal("detectLicense accepted an unknown license")
	}
}

func TestWithEnvironmentReplacesKeysCaseInsensitively(t *testing.T) {
	got := withEnvironment([]string{"Path=value", "GOOS=old", "OTHER=keep"}, "GOOS", "linux", "GOARCH", "arm64")
	want := map[string]bool{"Path=value": true, "OTHER=keep": true, "GOOS=linux": true, "GOARCH=arm64": true}
	if len(got) != len(want) {
		t.Fatalf("withEnvironment = %v", got)
	}
	for _, item := range got {
		if !want[item] {
			t.Errorf("unexpected environment entry %q", item)
		}
	}
}
