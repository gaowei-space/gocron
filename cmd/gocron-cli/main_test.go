package main

import (
	"flag"
	"net/url"
	"testing"

	"github.com/urfave/cli"
)

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

func TestTaskListValuesSupportsPaginationAliasesAndFilters(t *testing.T) {
	set := flag.NewFlagSet("task-list", flag.ContinueOnError)
	for _, item := range taskListFlags() {
		item.Apply(set)
	}
	args := []string{
		"--page", "2",
		"--pagesize", "50",
		"--name", "sync",
		"--host-id", "3",
		"--protocol", "2",
		"--tag", "ops",
		"--command", "php",
		"--status", "1",
	}
	if err := set.Parse(args); err != nil {
		t.Fatal(err)
	}

	values := taskListValues(cli.NewContext(cli.NewApp(), set, nil))
	assertValue(t, values.Get("page"), "2")
	assertValue(t, values.Get("page_size"), "50")
	assertValue(t, values.Get("name"), "sync")
	assertValue(t, values.Get("host_id"), "3")
	assertValue(t, values.Get("protocol"), "2")
	assertValue(t, values.Get("tag"), "ops")
	assertValue(t, values.Get("command"), "php")
	assertValue(t, values.Get("status"), "1")
}

func TestTaskListValuesPrefersPageSizeOverPagesize(t *testing.T) {
	set := flag.NewFlagSet("task-list", flag.ContinueOnError)
	for _, item := range taskListFlags() {
		item.Apply(set)
	}
	if err := set.Parse([]string{"--page-size", "20", "--pagesize", "50"}); err != nil {
		t.Fatal(err)
	}

	values := taskListValues(cli.NewContext(cli.NewApp(), set, nil))
	assertValue(t, values.Get("page_size"), "20")
}

func TestGetRequestIncludesQueryValues(t *testing.T) {
	values := url.Values{}
	values.Set("page", "2")
	values.Set("page_size", "50")

	endpoint := endpointWithQuery("https://gocron.example.com/api/agent/v1/tasks", values)
	assertValue(t, endpoint, "https://gocron.example.com/api/agent/v1/tasks?page=2&page_size=50")
}

func assertValue(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
