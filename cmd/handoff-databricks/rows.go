package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Citation struct {
	CheckpointID string `json:"checkpoint_id"`
	SessionIndex int    `json:"session_index"`
	Line         int    `json:"line,omitempty"`
}

type Item struct {
	Text  string     `json:"text"`
	Cites []Citation `json:"cites"`
}

type Section struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
	Note  string `json:"note,omitempty"`
}

type Packet struct {
	Repo        string    `json:"repo"`
	GeneratedAt time.Time `json:"generated_at"`
	Head        string    `json:"head_checkpoint_id"`
	Sections    []Section `json:"sections"`
}

type Row struct {
	RepoKey      string    `json:"repo_key"`
	CheckpointID string    `json:"checkpoint_id"`
	SessionID    int       `json:"session_id"`
	HeadID       string    `json:"head_checkpoint_id"`
	GeneratedAt  time.Time `json:"generated_at"`
	SectionName  string    `json:"section_name"`
	ItemIndex    int       `json:"item_index"`
	ItemText     string    `json:"item_text"`
	CiteLine     int       `json:"cite_line"`
	SectionNote  string    `json:"section_note"`
	IngestedAt   time.Time `json:"ingested_at"`
}

func Flatten(p Packet, ingestedAt time.Time) []Row {
	var rows []Row
	repoKey := strings.TrimSpace(p.Repo)
	if repoKey == "" {
		repoKey = "unknown"
	}

	for _, sec := range p.Sections {
		if len(sec.Items) == 0 {
			rows = append(rows, Row{
				RepoKey:     repoKey,
				HeadID:      p.Head,
				GeneratedAt: p.GeneratedAt,
				SectionName: sec.Name,
				ItemIndex:   -1,
				SectionNote: sec.Note,
				IngestedAt:  ingestedAt,
			})
			continue
		}
		for i, item := range sec.Items {
			cites := item.Cites
			if len(cites) == 0 {
				cites = []Citation{{}}
			}
			for _, c := range cites {
				rows = append(rows, Row{
					RepoKey:      repoKey,
					CheckpointID: c.CheckpointID,
					SessionID:    c.SessionIndex,
					HeadID:       p.Head,
					GeneratedAt:  p.GeneratedAt,
					SectionName:  sec.Name,
					ItemIndex:    i,
					ItemText:     item.Text,
					CiteLine:     c.Line,
					SectionNote:  sec.Note,
					IngestedAt:   ingestedAt,
				})
			}
		}
	}
	return rows
}

func EncodeNDJSON(rows []Row) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return nil, fmt.Errorf("encode row: %w", err)
		}
	}
	return []byte(b.String()), nil
}
