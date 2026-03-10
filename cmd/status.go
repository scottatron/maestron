package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Node-level summary",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	hostname, _ := os.Hostname()

	agentList, _ := discover.ListAgents(false)
	groups, _ := discover.ListSessions(discover.SessionFilter{Limit: 0})
	skills, _ := discover.ListSkills()
	mcpServers, _ := discover.ListMCPServers()

	sessionCount := 0
	for _, g := range groups {
		sessionCount += len(g.Sessions)
	}

	info := discover.NodeInfo{
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Agents:       agentList,
		SessionCount: sessionCount,
		SkillCount:   len(skills),
		MCPCount:     len(mcpServers),
		GeneratedAt:  time.Now(),
	}

	output.Print(info, func() {
		renderStatus(info, groups, skills, mcpServers)
	})
	return nil
}

func renderStatus(info discover.NodeInfo, groups []discover.SessionGroup, skills []discover.SkillInfo, mcpServers []discover.MCPServerInfo) {
	fmt.Printf("Node:     %s (%s)\n", info.Hostname, info.OS)
	fmt.Printf("Time:     %s\n", info.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	fmt.Printf("Agents:   %d\n", len(info.Agents))
	for _, a := range info.Agents {
		status := "inactive"
		if a.Active {
			status = "active"
		}
		fmt.Printf("  • %s %s (%s)\n", a.DisplayName, a.Version, status)
	}

	fmt.Println()
	fmt.Printf("Sessions: %d (across %d projects)\n", info.SessionCount, len(groups))
	for _, g := range groups {
		fmt.Printf("  • %s (%d sessions)\n", g.ProjectPath, len(g.Sessions))
	}

	fmt.Println()
	fmt.Printf("Skills:   %d\n", info.SkillCount)
	for _, s := range skills {
		fmt.Printf("  • %s [%s]\n", s.Name, s.Source)
	}

	fmt.Println()
	fmt.Printf("MCP:      %d servers\n", info.MCPCount)
	for _, m := range mcpServers {
		enabled := "enabled"
		if !m.Enabled {
			enabled = "disabled"
		}
		fmt.Printf("  • %s (%s)\n", m.Name, enabled)
	}
}
