package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli"
)

var (
	AppVersion = "1.6.4"
	BuildDate  string
	GitCommit  string
)

const defaultProfile = "default"

type config struct {
	Profiles map[string]*profile `json:"profiles"`
}

type profile struct {
	Server       string `json:"server"`
	DeviceId     string `json:"device_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type apiResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	RequestId string          `json:"request_id,omitempty"`
}

type apiError struct {
	Message   string
	RequestId string
}

func (e apiError) Error() string {
	if e.RequestId == "" {
		return e.Message
	}
	return e.Message + " (request_id: " + e.RequestId + ")"
}

func main() {
	app := cli.NewApp()
	app.Name = "gocron"
	app.Usage = "gocron command line client"
	app.Version = AppVersion
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "profile", Value: defaultProfile},
		cli.BoolFlag{Name: "json"},
	}
	app.Commands = []cli.Command{
		loginCommand(),
		logoutCommand(),
		taskCommand(),
		hostCommand(),
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loginCommand() cli.Command {
	return cli.Command{
		Name:  "login",
		Usage: "authorize this CLI device in browser",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "server", Usage: "gocron server base URL"},
			cli.StringFlag{Name: "device-name"},
			cli.StringFlag{Name: "client-version", Value: AppVersion},
		},
		Action: func(ctx *cli.Context) error {
			server := strings.TrimRight(ctx.String("server"), "/")
			if server == "" {
				return errors.New("--server is required")
			}
			deviceName := ctx.String("device-name")
			if deviceName == "" {
				hostname, _ := os.Hostname()
				deviceName = hostname
			}
			values := url.Values{}
			values.Set("device_name", deviceName)
			values.Set("client_type", "gocron-cli")
			values.Set("client_version", ctx.String("client-version"))
			resp, err := postForm(server+"/api/agent/v1/auth/device/start", "", values)
			if err != nil {
				return err
			}
			var data struct {
				DeviceCode              string `json:"device_code"`
				VerificationURIComplete string `json:"verification_uri_complete"`
				ExpiresIn               int64  `json:"expires_in"`
				Interval                int    `json:"interval"`
			}
			if err := decodeData(resp, &data); err != nil {
				return err
			}
			fmt.Printf("Open this URL in your browser and authorize the device:\n%s%s\n", server, data.VerificationURIComplete)
			deadline := time.Now().Add(time.Duration(data.ExpiresIn) * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(time.Duration(data.Interval) * time.Second)
				tokenResp, err := postForm(server+"/api/agent/v1/auth/device/token", "", url.Values{"device_code": {data.DeviceCode}})
				if err != nil {
					if strings.Contains(err.Error(), "授权待确认") {
						continue
					}
					return err
				}
				var tokenData struct {
					AccessToken  string `json:"access_token"`
					RefreshToken string `json:"refresh_token"`
					DeviceId     string `json:"device_id"`
				}
				if err := decodeData(tokenResp, &tokenData); err != nil {
					return err
				}
				cfg, _ := loadConfig()
				if cfg.Profiles == nil {
					cfg.Profiles = map[string]*profile{}
				}
				cfg.Profiles[ctx.GlobalString("profile")] = &profile{
					Server:       server,
					DeviceId:     tokenData.DeviceId,
					AccessToken:  tokenData.AccessToken,
					RefreshToken: tokenData.RefreshToken,
				}
				if err := saveConfig(cfg); err != nil {
					return err
				}
				fmt.Println("Login succeeded")
				return nil
			}
			return errors.New("authorization timed out")
		},
	}
}

func logoutCommand() cli.Command {
	return cli.Command{
		Name:  "logout",
		Usage: "revoke current device authorization",
		Action: func(ctx *cli.Context) error {
			cfg, prof, err := loadProfile(ctx.GlobalString("profile"))
			if err != nil {
				return err
			}
			_, _ = doRequest(prof, http.MethodPost, "/auth/logout", nil)
			delete(cfg.Profiles, ctx.GlobalString("profile"))
			return saveConfig(cfg)
		},
	}
}

func taskCommand() cli.Command {
	return cli.Command{
		Name: "task",
		Subcommands: []cli.Command{
			{Name: "list", Flags: taskListFlags(), Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				return printResponse(ctx, prof, http.MethodGet, "/tasks", taskListValues(ctx))
			})},
			{Name: "get", Usage: "get <id>", Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				return printResponse(ctx, prof, http.MethodGet, "/tasks/"+ctx.Args().First(), nil)
			})},
			{Name: "create", Flags: []cli.Flag{cli.StringFlag{Name: "file"}}, Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				values, err := valuesFromJSONFile(ctx.String("file"))
				if err != nil {
					return err
				}
				return printResponse(ctx, prof, http.MethodPost, "/tasks", values)
			})},
			{Name: "update", Usage: "update <id>", Flags: []cli.Flag{cli.StringFlag{Name: "file"}}, Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				values, err := valuesFromJSONFile(ctx.String("file"))
				if err != nil {
					return err
				}
				return printResponse(ctx, prof, http.MethodPut, "/tasks/"+ctx.Args().First(), values)
			})},
			{Name: "enable", Usage: "enable <id>", Action: simpleTaskPost("/enable")},
			{Name: "disable", Usage: "disable <id>", Action: simpleTaskPost("/disable")},
			{Name: "run", Usage: "run <id>", Action: simpleTaskPost("/run")},
			{Name: "logs", Usage: "logs <id>", Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				return printResponse(ctx, prof, http.MethodGet, "/tasks/"+ctx.Args().First()+"/logs", nil)
			})},
			{Name: "stop", Usage: "stop <task_id> <log_id>", Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				return printResponse(ctx, prof, http.MethodPost, "/tasks/"+ctx.Args().Get(0)+"/runs/"+ctx.Args().Get(1)+"/stop", nil)
			})},
		},
	}
}

func taskListFlags() []cli.Flag {
	return []cli.Flag{
		cli.IntFlag{Name: "page"},
		cli.IntFlag{Name: "page-size"},
		cli.IntFlag{Name: "pagesize"},
		cli.IntFlag{Name: "id"},
		cli.IntFlag{Name: "host-id"},
		cli.StringFlag{Name: "name"},
		cli.IntFlag{Name: "protocol"},
		cli.StringFlag{Name: "tag"},
		cli.StringFlag{Name: "command"},
		cli.IntFlag{Name: "status"},
	}
}

func taskListValues(ctx *cli.Context) url.Values {
	values := url.Values{}
	setIntFlag(values, ctx, "page", "page")
	pageSize := ctx.Int("page-size")
	if pageSize <= 0 {
		pageSize = ctx.Int("pagesize")
	}
	if pageSize > 0 {
		values.Set("page_size", strconv.Itoa(pageSize))
	}
	setIntFlag(values, ctx, "id", "id")
	setIntFlag(values, ctx, "host-id", "host_id")
	setStringFlag(values, ctx, "name", "name")
	setIntFlag(values, ctx, "protocol", "protocol")
	setStringFlag(values, ctx, "tag", "tag")
	setStringFlag(values, ctx, "command", "command")
	setIntFlag(values, ctx, "status", "status")
	return values
}

func setIntFlag(values url.Values, ctx *cli.Context, flagName, queryName string) {
	value := ctx.Int(flagName)
	if value > 0 {
		values.Set(queryName, strconv.Itoa(value))
	}
}

func setStringFlag(values url.Values, ctx *cli.Context, flagName, queryName string) {
	value := strings.TrimSpace(ctx.String(flagName))
	if value != "" {
		values.Set(queryName, value)
	}
}

func hostCommand() cli.Command {
	return cli.Command{
		Name: "host",
		Subcommands: []cli.Command{
			{Name: "list", Action: authedAction(func(ctx *cli.Context, prof *profile) error {
				return printResponse(ctx, prof, http.MethodGet, "/hosts", nil)
			})},
		},
	}
}

func simpleTaskPost(suffix string) func(*cli.Context) error {
	return authedAction(func(ctx *cli.Context, prof *profile) error {
		return printResponse(ctx, prof, http.MethodPost, "/tasks/"+ctx.Args().First()+suffix, nil)
	})
}

func authedAction(fn func(*cli.Context, *profile) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		_, prof, err := loadProfile(ctx.GlobalString("profile"))
		if err != nil {
			return err
		}
		if prof.AccessToken == "" {
			if err := refresh(prof); err != nil {
				return err
			}
		}
		return fn(ctx, prof)
	}
}

func printResponse(ctx *cli.Context, prof *profile, method, path string, values url.Values) error {
	resp, err := doRequest(prof, method, path, values)
	if err != nil {
		if refresh(prof) == nil {
			resp, err = doRequest(prof, method, path, values)
		}
	}
	if err != nil {
		return err
	}
	if ctx.GlobalBool("json") {
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
		return nil
	}
	fmt.Println(resp.Message)
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		fmt.Println(string(resp.Data))
	}
	return nil
}

func refresh(prof *profile) error {
	resp, err := postForm(prof.Server+"/api/agent/v1/auth/token/refresh", "", url.Values{"refresh_token": {prof.RefreshToken}})
	if err != nil {
		return err
	}
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeData(resp, &data); err != nil {
		return err
	}
	prof.AccessToken = data.AccessToken
	prof.RefreshToken = data.RefreshToken
	cfg, _ := loadConfig()
	for _, p := range cfg.Profiles {
		if p.DeviceId == prof.DeviceId {
			p.AccessToken = prof.AccessToken
			p.RefreshToken = prof.RefreshToken
		}
	}
	return saveConfig(cfg)
}

func doRequest(prof *profile, method, path string, values url.Values) (*apiResponse, error) {
	endpoint := prof.Server + "/api/agent/v1" + path
	if method == http.MethodGet {
		endpoint = endpointWithQuery(endpoint, values)
		req, err := http.NewRequest(method, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+prof.AccessToken)
		return send(req)
	}
	return formRequest(method, endpoint, prof.AccessToken, values)
}

func endpointWithQuery(endpoint string, values url.Values) string {
	if len(values) == 0 {
		return endpoint
	}
	return endpoint + "?" + values.Encode()
}

func postForm(endpoint, accessToken string, values url.Values) (*apiResponse, error) {
	return formRequest(http.MethodPost, endpoint, accessToken, values)
}

func formRequest(method, endpoint, accessToken string, values url.Values) (*apiResponse, error) {
	if values == nil {
		values = url.Values{}
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return send(req)
}

func send(req *http.Request) (*apiResponse, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid response: %s", string(body))
	}
	parsed.RequestId = resp.Header.Get("X-Request-Id")
	if parsed.Code != 0 {
		return nil, apiError{Message: parsed.Message, RequestId: parsed.RequestId}
	}
	return &parsed, nil
}

func decodeData(resp *apiResponse, out interface{}) error {
	if len(resp.Data) == 0 {
		return errors.New("empty response data")
	}
	return json.Unmarshal(resp.Data, out)
}

func valuesFromJSONFile(filename string) (url.Values, error) {
	if filename == "" {
		return nil, errors.New("--file is required")
	}
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal(body, &data); err != nil {
		return valuesFromSimpleYAML(body)
	}
	values := defaultTaskValues()
	for key, value := range data {
		switch v := value.(type) {
		case string:
			values.Set(key, v)
		case float64:
			values.Set(key, strconv.FormatInt(int64(v), 10))
		case bool:
			if v {
				values.Set(key, "1")
			} else {
				values.Set(key, "0")
			}
		}
	}
	return values, nil
}

func valuesFromSimpleYAML(body []byte) (url.Values, error) {
	values := defaultTaskValues()
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("unsupported YAML format")
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" {
			values.Set(key, value)
		}
	}
	return values, nil
}

func defaultTaskValues() url.Values {
	values := url.Values{}
	values.Set("tag", "")
	values.Set("level", "1")
	values.Set("dependency_status", "1")
	values.Set("dependency_task_id", "")
	values.Set("spec", "")
	values.Set("protocol", "2")
	values.Set("http_method", "1")
	values.Set("host_id", "")
	values.Set("timeout", "0")
	values.Set("multi", "2")
	values.Set("notify_status", "1")
	values.Set("notify_type", "2")
	values.Set("notify_receiver_id", "")
	values.Set("notify_keyword", "")
	values.Set("retry_times", "0")
	values.Set("retry_interval", "0")
	values.Set("status", "1")
	values.Set("remark", "")
	return values
}

func loadProfile(name string) (*config, *profile, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	prof := cfg.Profiles[name]
	if prof == nil {
		return nil, nil, errors.New("not logged in")
	}
	return cfg, prof, nil
}

func loadConfig() (*config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	cfg := &config{Profiles: map[string]*profile{}}
	body, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, cfg); err != nil {
		return nil, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*profile{}
	}
	return cfg, nil
}

func saveConfig(cfg *config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, body, 0600)
}

func configPath() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(current.HomeDir, ".gocron", "config.json"), nil
}
