// workspaces.go

package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hashicorp/go-tfe"
	"github.com/queone/gkit/color"
)

// listAllWorkspaces walks every page of the organization's workspaces.
func listAllWorkspaces(ctx context.Context, a api, org string) ([]*tfe.Workspace, error) {
	var all []*tfe.Workspace
	for page := 1; page != 0; {
		items, nextPage, err := a.ListWorkspaces(ctx, org, page)
		if err != nil {
			return nil, fmt.Errorf("listing workspaces for organization %s: %w", org, err)
		}
		all = append(all, items...)
		page = nextPage
	}
	return all, nil
}

// listAllVariables walks every page of a workspace's variables.
func listAllVariables(ctx context.Context, a api, workspaceID, name string) ([]*tfe.Variable, error) {
	var all []*tfe.Variable
	for page := 1; page != 0; {
		items, nextPage, err := a.ListVariables(ctx, workspaceID, page)
		if err != nil {
			return nil, fmt.Errorf("listing variables for workspace %s: %w", name, err)
		}
		all = append(all, items...)
		page = nextPage
	}
	return all, nil
}

// listWorkspaces prints every workspace whose name matches filter, one per line.
func listWorkspaces(ctx context.Context, a api, org, filter string, w io.Writer) error {
	all, err := listAllWorkspaces(ctx, a, org)
	if err != nil {
		return err
	}
	for _, ws := range all {
		if matches(ws.Name, filter) {
			fmt.Fprintln(w, ws.Name)
		}
	}
	return nil
}

// orEmpty renders an empty string as a quoted empty value.
func orEmpty(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

// showWorkspace prints one workspace's attributes, its agent pool when it runs
// in agent mode, and its variables grouped by category.
func showWorkspace(ctx context.Context, a api, org, name string, w io.Writer) error {
	ws, err := a.ReadWorkspace(ctx, org, name)
	if err != nil {
		return fmt.Errorf("reading workspace %s in organization %s: %w", name, org, err)
	}
	k, v := color.Blu8, color.Grn5
	fmt.Fprintln(w, color.Gra5("# Terraform Cloud Workspace"))
	fmt.Fprintf(w, "%s: %s\n", k("workspace_name"), v(ws.Name))
	fmt.Fprintf(w, "%s: %s\n", k("workspace_id"), v(ws.ID))
	fmt.Fprintf(w, "%s: %s\n", k("created_at"), v(ws.CreatedAt.Format("2006-Jan-02 15:04")))
	fmt.Fprintf(w, "%s: %s\n", k("updated_at"), v(ws.UpdatedAt.Format("2006-Jan-02 15:04")))
	fmt.Fprintf(w, "%s: %s\n", k("description"), v(orEmpty(ws.Description)))
	fmt.Fprintf(w, "%s: %s\n", k("terraform_version"), v(ws.TerraformVersion))
	fmt.Fprintf(w, "%s: %s\n", k("auto_apply"), v(strconv.FormatBool(ws.AutoApply)))
	fmt.Fprintf(w, "%s: %s\n", k("working_directory"), v(orEmpty(ws.WorkingDirectory)))
	fmt.Fprintf(w, "%s: %s\n", k("execution_mode"), v(ws.ExecutionMode))
	if ws.ExecutionMode == "agent" {
		if ws.AgentPool == nil {
			fmt.Fprintf(w, "  %s: %s\n", k("agent_pool_id"), v("Not available"))
		} else {
			pool, err := a.ReadAgentPool(ctx, ws.AgentPool.ID)
			if err != nil {
				return fmt.Errorf("reading agent pool %s for workspace %s: %w", ws.AgentPool.ID, name, err)
			}
			fmt.Fprintf(w, "  %s: %s\n", k("agent_pool_name"), v(pool.Name))
		}
	}
	vars, err := listAllVariables(ctx, a, ws.ID, name)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s:\n", k("variables"))
	for _, group := range []struct {
		label    string
		category tfe.CategoryType
	}{{"environment", tfe.CategoryEnv}, {"terraform", tfe.CategoryTerraform}} {
		var rows []*tfe.Variable
		for _, x := range vars {
			if x.Category == group.category {
				rows = append(rows, x)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s:\n", k(group.label))
		for _, x := range rows {
			fmt.Fprintf(w, "    %s: %s\n", k(x.Key), v(x.Value))
		}
	}
	return nil
}

// cloneWorkspace creates dest with src's settings and copies every variable.
// The API never returns sensitive values, so those variables arrive empty and
// are named in a warning for the user to fill in.
func cloneWorkspace(ctx context.Context, a api, org, src, dest string, w io.Writer) error {
	source, err := a.ReadWorkspace(ctx, org, src)
	if err != nil {
		return fmt.Errorf("reading source workspace %s in organization %s: %w", src, org, err)
	}
	options := tfe.WorkspaceCreateOptions{
		Name:             new(dest),
		AutoApply:        new(source.AutoApply),
		TerraformVersion: new(source.TerraformVersion),
		WorkingDirectory: new(source.WorkingDirectory),
		Description:      new(source.Description),
		ExecutionMode:    new(source.ExecutionMode),
	}
	if source.ExecutionMode == "agent" && source.AgentPool != nil {
		options.AgentPoolID = new(source.AgentPool.ID)
	}
	target, err := a.CreateWorkspace(ctx, org, options)
	if err != nil {
		return fmt.Errorf("creating workspace %s in organization %s: %w", dest, org, err)
	}
	vars, err := listAllVariables(ctx, a, source.ID, src)
	if err != nil {
		return err
	}
	var sensitive []string
	for _, x := range vars {
		_, err := a.CreateVariable(ctx, target.ID, tfe.VariableCreateOptions{
			Key:       new(x.Key),
			Value:     new(x.Value),
			Category:  new(x.Category),
			HCL:       new(x.HCL),
			Sensitive: new(x.Sensitive),
		})
		if err != nil {
			return fmt.Errorf("creating variable %s in workspace %s: %w", x.Key, dest, err)
		}
		if x.Sensitive {
			sensitive = append(sensitive, x.Key)
		}
	}
	fmt.Fprintf(w, "Cloned workspace %s to %s\n", color.Blu8(src), color.Grn5(dest))
	if len(sensitive) > 0 {
		fmt.Fprintln(w, color.Yel5("Sensitive variables were copied with empty values; set them again in "+dest+": "+strings.Join(sensitive, ", ")))
	}
	return nil
}
