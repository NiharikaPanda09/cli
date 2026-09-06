package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func samplePacket(t *testing.T) Packet {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "packet.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var p Packet
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return p
}

func TestFlattenProducesARowPerCitation(t *testing.T) {
	t.Parallel()

	rows := Flatten(samplePacket(t), time.Unix(0, 0).UTC())
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	var dead int
	for _, r := range rows {
		if r.RepoKey == "" {
			t.Error("row has empty repo_key")
		}
		if r.SectionName == "dead_ends" && r.ItemIndex >= 0 {
			dead++
			if r.CheckpointID == "" {
				t.Error("dead end row lost its checkpoint id")
			}
			if r.CiteLine == 0 {
				t.Error("dead end row lost its cite line")
			}
		}
	}
	if dead == 0 {
		t.Fatal("expected at least one dead_ends row")
	}
}

func TestFlattenKeepsEmptySectionsWithTheirNote(t *testing.T) {
	t.Parallel()

	rows := Flatten(samplePacket(t), time.Unix(0, 0).UTC())
	for _, r := range rows {
		if r.SectionName == "surface" {
			if r.ItemIndex != -1 {
				t.Errorf("empty section should carry ItemIndex -1, got %d", r.ItemIndex)
			}
			if r.SectionNote == "" {
				t.Error("empty section lost its note")
			}
			return
		}
	}
	t.Fatal("empty surface section was dropped entirely")
}

func TestFlattenFallsBackWhenRepoMissing(t *testing.T) {
	t.Parallel()

	rows := Flatten(Packet{Sections: []Section{{Name: "intent", Items: []Item{{Text: "x", Cites: []Citation{{CheckpointID: "c"}}}}}}}, time.Unix(0, 0))
	if len(rows) != 1 || rows[0].RepoKey != "unknown" {
		t.Fatalf("want repo_key 'unknown', got %+v", rows)
	}
}

func TestEncodeNDJSONIsOneObjectPerLine(t *testing.T) {
	t.Parallel()

	rows := Flatten(samplePacket(t), time.Unix(0, 0).UTC())
	payload, err := EncodeNDJSON(rows)
	if err != nil {
		t.Fatalf("EncodeNDJSON: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	if len(lines) != len(rows) {
		t.Fatalf("want %d lines, got %d", len(rows), len(lines))
	}
	for i, l := range lines {
		var r Row
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestUploadWritesLocallyWhenHostIsFileScheme(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Host: "file://" + dir}

	u := NewUploader(cfg)
	p := samplePacket(t)
	dest, err := u.Upload(context.Background(), p, []byte("{}\n"), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected local file at %s: %v", dest, err)
	}
	if err := u.CopyInto(context.Background(), dest); err != nil {
		t.Fatalf("CopyInto must be a no-op locally: %v", err)
	}
}

func TestUploadPutsToFilesAPIWithBearer(t *testing.T) {
	t.Parallel()

	var gotPath, gotAuth, gotMethod string
	var gotBody []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = readAll(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u := &Uploader{
		cfg:    Config{Host: srv.URL, Token: "secret-token", VolumePath: "/Volumes/main/default/handoff"},
		client: srv.Client(),
	}
	p := samplePacket(t)
	if _, err := u.Upload(context.Background(), p, []byte("row\n"), time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if !strings.HasPrefix(gotPath, "/api/2.0/fs/files/Volumes/main/default/handoff/") {
		t.Errorf("path = %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if !bytes.Equal(gotBody, []byte("row\n")) {
		t.Errorf("body = %q", gotBody)
	}
}

func TestCopyIntoStatementCarriesNoPacketText(t *testing.T) {
	t.Parallel()

	var stmt string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := readAll(r)
		_ = json.Unmarshal(raw, &body)
		stmt, _ = body["statement"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u := &Uploader{
		cfg:    Config{Host: srv.URL, Token: "t", WarehouseID: "wh1", VolumePath: "/Volumes/x", Table: "handoff_packets"},
		client: srv.Client(),
	}
	if err := u.CopyInto(context.Background(), "/Volumes/x/repo/head-1.json"); err != nil {
		t.Fatalf("CopyInto: %v", err)
	}
	if !strings.Contains(stmt, "COPY INTO handoff_packets") {
		t.Errorf("statement = %q", stmt)
	}
	if strings.Contains(stmt, "recap") || strings.Contains(stmt, "undefined") {
		t.Errorf("packet text leaked into SQL: %q", stmt)
	}
}

func TestUploadRejectsPlainHTTPHost(t *testing.T) {
	t.Parallel()

	u := NewUploader(Config{Host: "http://example.com", Token: "t", VolumePath: "/v"})
	if _, err := u.Upload(context.Background(), Packet{}, []byte("x"), time.Unix(0, 0)); err == nil {
		t.Fatal("plain http host must be rejected")
	}
}

func TestUploadSurfacesServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"denied"}`))
	}))
	defer srv.Close()

	u := &Uploader{cfg: Config{Host: srv.URL, Token: "t", VolumePath: "/v"}, client: srv.Client()}
	_, err := u.Upload(context.Background(), Packet{}, []byte("x"), time.Unix(0, 0))
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want the server message surfaced, got %v", err)
	}
}

func TestSanitizeSegmentStripsPathTraversal(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"../../etc/passwd", "gh/acme/cli", ""} {
		got := sanitizeSegment(in)
		if strings.Contains(got, "/") || strings.Contains(got, "..") {
			t.Errorf("sanitizeSegment(%q) = %q, still traversable", in, got)
		}
	}
}

func readAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := &bytes.Buffer{}
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}
