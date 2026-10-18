// Command opnsense-assignment-probe records release availability of the
// OPNsense interfaces/assignment API used by the provider roadmap.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBase = "https://api.github.com"
	modelPath      = "src/opnsense/mvc/app/models/OPNsense/Interfaces/NetworkInterface.xml"
	controllerPath = "src/opnsense/mvc/app/controllers/OPNsense/Interfaces/Api/AssignmentController.php"
)

var requiredFields = []string{
	"type4",
	"ipaddr",
	"type6",
	"ipaddrv6",
	"gateway",
	"gatewayv6",
	"dhcphostname",
	"dhcprejectfrom",
	"dhcphonourmtu",
	"dhcpvlanprio",
	"dhcp_send_options",
	"dhcp_request_options",
	"dhcp6vlanprio",
	"dhcp6_request_dns",
	"dhcp6-information-only",
	"dhcp6_rapid_commit",
	"dhcp6_send_options",
	"dhcp6_request_options",
}

type ref struct {
	Name string `json:"name"`
	SHA  string `json:"sha,omitempty"`
	Kind string `json:"kind"`
}

type result struct {
	Ref             ref             `json:"ref"`
	ModelURL        string          `json:"model_url"`
	ModelSHA        string          `json:"model_sha,omitempty"`
	ControllerURL   string          `json:"controller_url"`
	ControllerSHA   string          `json:"controller_sha,omitempty"`
	Fields          map[string]bool `json:"fields"`
	SetItem         bool            `json:"set_item"`
	Reconfigure     bool            `json:"reconfigure"`
	Complete        bool            `json:"complete"`
	InspectionError string          `json:"inspection_error,omitempty"`
}

type report struct {
	SchemaVersion    int      `json:"schema_version"`
	Upstream         string   `json:"upstream"`
	RequiredFields   []string `json:"required_fields"`
	FirstReleasedTag string   `json:"first_released_tag,omitempty"`
	Results          []result `json:"results"`
}

type githubRef struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type contentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

type client struct {
	apiBase    string
	repository string
	token      string
	http       *http.Client
}

func main() {
	var (
		repository   = flag.String("repository", "opnsense/core", "GitHub owner/repository to inspect")
		output       = flag.String("output", "-", "output path, or - for stdout")
		stableLimit  = flag.Int("stable-limit", 2, "number of newest stable branches to inspect")
		tagLimit     = flag.Int("tag-limit", 12, "number of newest release tags to inspect")
		boundaryTags = flag.String(
			"boundary-tags",
			"26.7.5,26.7.6",
			"comma-separated release boundary tags that must remain in the probe",
		)
		apiBase = flag.String("api-base", defaultAPIBase, "GitHub API base URL")
	)
	flag.Parse()

	if *stableLimit < 1 || *tagLimit < 1 {
		fatal(errors.New("stable-limit and tag-limit must be positive"))
	}

	c := &client{
		apiBase:    strings.TrimRight(*apiBase, "/"),
		repository: *repository,
		token:      os.Getenv("GITHUB_TOKEN"),
		http:       &http.Client{Timeout: 30 * time.Second},
	}

	ctx := context.Background()
	report, err := c.probe(ctx, *stableLimit, *tagLimit, splitCSV(*boundaryTags))
	if err != nil {
		fatal(err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(fmt.Errorf("encode report: %w", err))
	}
	data = append(data, '\n')

	if *output == "-" {
		_, err = os.Stdout.Write(data)
	} else {
		err = os.WriteFile(*output, data, 0o600)
	}
	if err != nil {
		fatal(fmt.Errorf("write report: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "opnsense-assignment-probe: %v\n", err)
	os.Exit(1)
}

func (c *client) probe(ctx context.Context, stableLimit, tagLimit int, boundaryTags []string) (report, error) {
	branches, err := c.listRefs(ctx, "branches")
	if err != nil {
		return report{}, err
	}
	tags, err := c.listRefs(ctx, "tags")
	if err != nil {
		return report{}, err
	}

	refs := []ref{}
	for _, branch := range branches {
		if branch.Name == "master" {
			refs = append(refs, ref{Name: branch.Name, Kind: "branch"})
			break
		}
	}
	for _, branch := range selectStableBranches(branches, stableLimit) {
		refs = append(refs, ref{Name: branch.Name, Kind: "stable-branch"})
	}
	for _, tag := range selectReleaseTags(tags, tagLimit, boundaryTags) {
		refs = append(refs, ref{Name: tag.Name, SHA: tag.Commit.SHA, Kind: "tag"})
	}

	results := make([]result, 0, len(refs))
	for _, item := range refs {
		model, modelSHA, modelErr := c.getContent(ctx, item.Name, modelPath)
		controller, controllerSHA, controllerErr := c.getContent(ctx, item.Name, controllerPath)
		results = append(results, inspect(
			item,
			c.repository,
			model,
			modelSHA,
			controller,
			controllerSHA,
			errors.Join(modelErr, controllerErr),
		))
	}

	firstReleased := ""
	for _, item := range results {
		if item.Ref.Kind != "tag" || !item.Complete {
			continue
		}
		if firstReleased == "" || compareVersions(item.Ref.Name, firstReleased) < 0 {
			firstReleased = item.Ref.Name
		}
	}

	return report{
		SchemaVersion:    1,
		Upstream:         c.repository,
		RequiredFields:   append([]string(nil), requiredFields...),
		FirstReleasedTag: firstReleased,
		Results:          results,
	}, nil
}

func (c *client) listRefs(ctx context.Context, kind string) ([]githubRef, error) {
	requestURL := fmt.Sprintf("%s/repos/%s/%s?per_page=100", c.apiBase, c.repository, kind)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", kind, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list %s: HTTP %d: %s", kind, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var refs []githubRef
	if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
		return nil, fmt.Errorf("decode %s: %w", kind, err)
	}
	return refs, nil
}

func (c *client) getContent(ctx context.Context, refName, path string) (string, string, error) {
	requestURL := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", c.apiBase, c.repository, path, url.QueryEscape(refName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", "", err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch %s at %s: %w", path, refName, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("fetch %s at %s: HTTP %d", path, refName, resp.StatusCode)
	}

	var payload contentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", fmt.Errorf("decode %s at %s: %w", path, refName, err)
	}
	if payload.Encoding != "base64" {
		return "", "", fmt.Errorf("decode %s at %s: unsupported encoding %q", path, refName, payload.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return "", "", fmt.Errorf("decode %s at %s: %w", path, refName, err)
	}
	return string(decoded), payload.SHA, nil
}

func (c *client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func inspect(item ref, repository, model, modelSHA, controller, controllerSHA string, inspectionErr error) result {
	fields := make(map[string]bool, len(requiredFields))
	complete := inspectionErr == nil
	for _, field := range requiredFields {
		fields[field] = strings.Contains(model, "<"+field)
		complete = complete && fields[field]
	}
	setItem := strings.Contains(controller, "function setItemAction")
	reconfigure := strings.Contains(controller, "function reconfigureAction")
	complete = complete && setItem && reconfigure

	result := result{
		Ref:           item,
		ModelURL:      fmt.Sprintf("https://github.com/%s/blob/%s/%s", repository, item.Name, modelPath),
		ModelSHA:      modelSHA,
		ControllerURL: fmt.Sprintf("https://github.com/%s/blob/%s/%s", repository, item.Name, controllerPath),
		ControllerSHA: controllerSHA,
		Fields:        fields,
		SetItem:       setItem,
		Reconfigure:   reconfigure,
		Complete:      complete,
	}
	if inspectionErr != nil {
		result.InspectionError = inspectionErr.Error()
	}
	return result
}

func selectStableBranches(refs []githubRef, limit int) []githubRef {
	selected := make([]githubRef, 0)
	for _, item := range refs {
		if strings.HasPrefix(item.Name, "stable/") && parseVersion(strings.TrimPrefix(item.Name, "stable/")).valid {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return compareVersions(strings.TrimPrefix(selected[i].Name, "stable/"), strings.TrimPrefix(selected[j].Name, "stable/")) > 0
	})
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func selectReleaseTags(refs []githubRef, limit int, boundaryTags []string) []githubRef {
	selected := make([]githubRef, 0)
	byName := make(map[string]githubRef, len(refs))
	for _, item := range refs {
		byName[item.Name] = item
		if parseVersion(item.Name).valid {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return compareVersions(selected[i].Name, selected[j].Name) > 0
	})
	if len(selected) > limit {
		selected = selected[:limit]
	}
	present := make(map[string]bool, len(selected))
	for _, item := range selected {
		present[item.Name] = true
	}
	for _, name := range boundaryTags {
		if item, ok := byName[name]; ok && !present[name] {
			selected = append(selected, item)
			present[name] = true
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return compareVersions(selected[i].Name, selected[j].Name) > 0
	})
	return selected
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?(?:\.([a-z]+)(\d*)?)?$`)

type version struct {
	major, minor, patch int
	channel, sequence   int
	valid               bool
}

func parseVersion(value string) version {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return version{}
	}
	result := version{valid: true, channel: 4}
	result.major, _ = strconv.Atoi(match[1])
	result.minor, _ = strconv.Atoi(match[2])
	result.patch, _ = strconv.Atoi(match[3])
	if match[4] != "" {
		result.channel = map[string]int{"a": 1, "b": 2, "r": 3}[match[4]]
		if result.channel == 0 {
			return version{}
		}
		result.sequence, _ = strconv.Atoi(match[5])
	}
	return result
}

func compareVersions(left, right string) int {
	a, b := parseVersion(left), parseVersion(right)
	av := []int{a.major, a.minor, a.patch, a.channel, a.sequence}
	bv := []int{b.major, b.minor, b.patch, b.channel, b.sequence}
	for i := range av {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}
