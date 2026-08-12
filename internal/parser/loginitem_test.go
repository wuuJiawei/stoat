package parser

import "testing"

func TestParseBTMDump(t *testing.T) {
	dump := []byte(`#1:
UUID: 1234
Name: Example Helper
Developer Name: Example Inc.
Type: background item
Identifier: com.example.helper
URL: file:///Applications/Example.app

#2:
UUID: 5678
Name: Login App
Type: login item
Identifier: com.example.login
URL: file:///Applications/Login%20App.app
`)
	items := ParseBTMDump(dump)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].BundleID != "com.example.helper" || !items[0].HasCategory("background") {
		t.Fatalf("unexpected background item: %#v", items[0])
	}
	if items[1].Program != "/Applications/Login App.app" {
		t.Fatalf("unexpected decoded path: %s", items[1].Program)
	}
}
