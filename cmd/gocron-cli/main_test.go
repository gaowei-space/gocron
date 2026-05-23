package main

import "testing"

func TestValuesFromSimpleYAMLUsesWebDefaults(t *testing.T) {
	values, err := valuesFromSimpleYAML([]byte(`
name: nightly-sync
spec: "0 0 2 * * *"
command: "php /data/app/index.php sync/run"
host_id: "1"
`))
	if err != nil {
		t.Fatal(err)
	}

	assertValue(t, values.Get("name"), "nightly-sync")
	assertValue(t, values.Get("command"), "php /data/app/index.php sync/run")
	assertValue(t, values.Get("level"), "1")
	assertValue(t, values.Get("dependency_status"), "1")
	assertValue(t, values.Get("protocol"), "2")
	assertValue(t, values.Get("http_method"), "1")
	assertValue(t, values.Get("timeout"), "0")
	assertValue(t, values.Get("multi"), "2")
	assertValue(t, values.Get("notify_status"), "1")
	assertValue(t, values.Get("notify_type"), "2")
	assertValue(t, values.Get("retry_times"), "0")
	assertValue(t, values.Get("retry_interval"), "0")
	assertValue(t, values.Get("status"), "1")
}

func TestValuesFromSimpleYAMLOverridesWebDefaults(t *testing.T) {
	values, err := valuesFromSimpleYAML([]byte(`
name: healthcheck
level: 2
protocol: 1
http_method: 2
command: "https://example.com/health"
status: 0
`))
	if err != nil {
		t.Fatal(err)
	}

	assertValue(t, values.Get("level"), "2")
	assertValue(t, values.Get("protocol"), "1")
	assertValue(t, values.Get("http_method"), "2")
	assertValue(t, values.Get("status"), "0")
}

func assertValue(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
