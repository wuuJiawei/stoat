package attribution

import "testing"

func TestAppBundleForPath(t *testing.T) {
	tests := map[string]string{
		"/Applications/Example.app":                                     "/Applications/Example.app",
		"/Applications/Example.app/Contents/MacOS/example":              "/Applications/Example.app",
		"/Applications/Outer.app/Contents/Helpers/Inner.app/Contents/x": "/Applications/Outer.app/Contents/Helpers/Inner.app",
		"/usr/local/bin/example":                                        "",
		"relative/Example.app":                                          "",
	}
	for input, expected := range tests {
		if actual := AppBundleForPath(input); actual != expected {
			t.Errorf("AppBundleForPath(%q) = %q, want %q", input, actual, expected)
		}
	}
}
