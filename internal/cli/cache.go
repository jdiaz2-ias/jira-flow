package cli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"os"
	"path/filepath"
	"runtime"
)

func diskFor(a app.Access, name string) (*cache.Disk, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Could not resolve cache location."}
	}
	if p := a.Env("JFLOW_CACHE"); p != "" && !filepath.IsAbs(p) {
		return nil, &domain.Error{Kind: domain.InvalidInput, Message: "JFLOW_CACHE must be an absolute path."}
	}
	paths := config.ResolvePaths(runtime.GOOS, home, a.Env)
	configPath, err := filepath.Abs(a.Path)
	if err != nil {
		return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Could not resolve configuration location."}
	}
	return cache.NewDisk(paths.Cache, configPath+"\x00"+name), nil
}
func cacheError(err error) error {
	if err == nil {
		return nil
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return &domain.Error{Kind: domain.Canceled, Message: "Cache operation canceled."}
	}
	return &domain.Error{Kind: domain.Unavailable, Message: "Could not access the private cache. Check directory permissions and retry.", Cause: err}
}
func addCache(root *cobra.Command, access func() (app.Access, error), profile *string, memory *cache.Memory, emit func(output.Envelope, string, error) error) {
	group := &cobra.Command{Use: "cache", Short: "Inspect or clear this profile's private persistent cache"}
	for _, mode := range []string{"status", "clear"} {
		cmd := &cobra.Command{Use: mode, Short: map[string]string{"status": "Show cache usage and persistence setting", "clear": "Remove cached results for the selected profile"}[mode], Args: cobra.NoArgs}
		cmd.RunE = func(cmd *cobra.Command, _ []string) error {
			a, err := access()
			if err != nil {
				return err
			}
			c, err := config.Load(a.Path)
			if err != nil {
				return err
			}
			name, p, err := c.Select(*profile, a.Env)
			if err != nil {
				return err
			}
			disk, err := diskFor(a, name)
			if err != nil {
				return err
			}
			if mode == "clear" {
				if err := advanceCacheGeneration(cmd.Context(), a, name); err != nil {
					return err
				}
				if err := disk.Clear(cmd.Context()); err != nil {
					return cacheError(err)
				}
				memory.DeletePrefix("")
				return emit(output.Success(map[string]any{"profile": name, "cleared": true}), "Cache cleared for profile "+name+".", nil)
			}
			stats, err := disk.Status(cmd.Context())
			if err != nil {
				return cacheError(err)
			}
			return emit(output.Success(map[string]any{"profile": name, "persist": p.Cache.Persist, "path": disk.Dir, "usage": stats, "max_bytes": 25 * 1024 * 1024, "max_age_days": 7}), fmt.Sprintf("Profile: %s\nPersistence: %t\nCache: %s\nEntries: %d · bytes: %d · stale: %d", name, p.Cache.Persist, disk.Dir, stats.Entries, stats.Bytes, stats.Stale), nil)
		}
		group.AddCommand(cmd)
	}
	root.AddCommand(group)
}
func advanceCacheGeneration(ctx context.Context, a app.Access, name string) error {
	return config.Update(ctx, a.Path, func(c *config.Config) error {
		p, ok := c.Profiles[name]
		if !ok {
			return &domain.Error{Kind: domain.Conflict, Message: "Profile changed during cache invalidation."}
		}
		p.CacheGeneration++
		c.Profiles[name] = p
		return nil
	})
}
