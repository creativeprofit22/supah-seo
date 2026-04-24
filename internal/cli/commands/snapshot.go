package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewSnapshotCmd returns the snapshot command group.
func NewSnapshotCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Save and list point-in-time copies of the audit state",
		Long: `Snapshots copy the current .supah-seo/state.json into
.supah-seo/snapshots/<timestamp>[-<label>].json so they can be compared later
with 'supah-seo report compare'. Useful for tracking improvement over 30, 60,
and 90 day windows on a retainer client.`,
	}

	cmd.AddCommand(
		newSnapshotCreateCmd(format, verbose),
		newSnapshotListCmd(format, verbose),
	)
	return cmd
}

func newSnapshotCreateCmd(format *string, verbose *bool) *cobra.Command {
	var label string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Copy the current state file into the snapshots directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			src := filepath.Join(state.DirName, state.FileName)
			if _, err := os.Stat(src); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "no state file found; run an audit first", err, map[string]any{"path": src}, output.Format(*format))
			}
			dstDir := filepath.Join(state.DirName, "snapshots")
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to create snapshots dir", err, nil, output.Format(*format))
			}

			ts := time.Now().Format("2006-01-02T15-04-05")
			name := ts
			if l := sanitizeLabel(label); l != "" {
				name = ts + "-" + l
			}
			dst := filepath.Join(dstDir, name+".json")
			if err := copyFile(src, dst); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to copy state", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"snapshot": dst,
				"label":    label,
				"taken_at": time.Now().Format(time.RFC3339),
			}, map[string]any{"verbose": *verbose}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Optional label suffix (e.g. 'baseline', 'month-1', 'post-fix')")
	return cmd
}

func newSnapshotListCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available snapshots for the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(state.DirName, "snapshots")
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					return output.PrintSuccess([]any{}, map[string]any{
						"count":   0,
						"verbose": *verbose,
					}, output.Format(*format))
				}
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to read snapshots dir", err, nil, output.Format(*format))
			}

			type entry struct {
				Name    string `json:"name"`
				Path    string `json:"path"`
				Label   string `json:"label,omitempty"`
				TakenAt string `json:"taken_at"`
				SizeKB  int    `json:"size_kb"`
			}
			var list []entry
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				full := filepath.Join(dir, e.Name())
				info, err := e.Info()
				if err != nil {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".json")
				ts, label := parseSnapshotName(name)
				list = append(list, entry{
					Name:    name,
					Path:    full,
					Label:   label,
					TakenAt: ts,
					SizeKB:  int(info.Size() / 1024),
				})
			}
			sort.SliceStable(list, func(i, j int) bool {
				return list[i].TakenAt > list[j].TakenAt
			})

			return output.PrintSuccess(list, map[string]any{
				"count":   len(list),
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

// --- helpers ---

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func sanitizeLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prev := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		case r == ' ' || r == '-' || r == '_':
			if !prev && b.Len() > 0 {
				b.WriteByte('-')
				prev = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// parseSnapshotName splits a snapshot filename stem into its timestamp and optional label.
// Filename format: <ts>[-<label>] where <ts> = 2006-01-02T15-04-05.
func parseSnapshotName(name string) (ts, label string) {
	// Timestamp is 19 chars: 2006-01-02T15-04-05
	const tsLen = 19
	if len(name) < tsLen {
		return name, ""
	}
	ts = name[:tsLen]
	if len(name) > tsLen && name[tsLen] == '-' {
		label = name[tsLen+1:]
	}
	if _, err := fmt.Sscanf(ts, "2006-01-02T15-04-05"); err == nil {
		return ts, label
	}
	// Fallback: no strict format parse here, just return as-is.
	return ts, label
}
