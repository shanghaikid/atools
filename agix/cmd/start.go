package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agent-platform/agix/internal/alert"
	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/dashboard"
	"github.com/agent-platform/agix/internal/failover"
	"github.com/agent-platform/agix/internal/firewall"
	"github.com/agent-platform/agix/internal/proxy"
	"github.com/agent-platform/agix/internal/ratelimit"
	"github.com/agent-platform/agix/internal/router"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/spf13/cobra"
)

var startPort int

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent gateway",
	Long: `Starts the gateway that sits between your agents and LLM providers.
Tracks usage and costs, enforces budgets, and provides shared MCP tools.

Agents should point their API base URL to http://localhost:<port>.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		if startPort != 0 {
			cfg.Port = startPort
		}

		// Open store
		st, err := store.New(cfg.Database)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer st.Close()

		// Initialize tool manager (if MCP servers are configured)
		toolMgr, err := initToolManager(cfg)
		if err != nil {
			return fmt.Errorf("initialize tool manager: %w", err)
		}
		if toolMgr != nil {
			defer toolMgr.Close()
		}

		// Build proxy options
		var proxyOpts []proxy.Option
		if toolMgr != nil {
			proxyOpts = append(proxyOpts, proxy.WithToolManager(toolMgr))
		}

		// Initialize rate limiter
		if len(cfg.RateLimits) > 0 {
			limits := make(map[string]ratelimit.Limit, len(cfg.RateLimits))
			for agent, rl := range cfg.RateLimits {
				limits[agent] = ratelimit.Limit{
					RequestsPerMinute: rl.RequestsPerMinute,
					RequestsPerHour:   rl.RequestsPerHour,
				}
			}
			proxyOpts = append(proxyOpts, proxy.WithRateLimiter(ratelimit.New(limits)))
		}

		// Initialize failover
		if len(cfg.Failover.Chains) > 0 {
			f := failover.New(failover.Config{
				MaxRetries: cfg.Failover.MaxRetries,
				Chains:     cfg.Failover.Chains,
			})
			if f != nil {
				proxyOpts = append(proxyOpts, proxy.WithFailover(f))
			}
		}

		// Initialize alerter for budget webhooks
		proxyOpts = append(proxyOpts, proxy.WithAlerter(alert.NewAlerter(5*time.Minute)))

		// Initialize firewall
		if cfg.Firewall.Enabled {
			var rules []firewall.RuleConfig
			for _, r := range cfg.Firewall.Rules {
				rules = append(rules, firewall.RuleConfig{
					Name:     r.Name,
					Category: r.Category,
					Pattern:  r.Pattern,
					Action:   firewall.Action(r.Action),
				})
			}
			fw, err := firewall.New(firewall.Config{
				Enabled: true,
				Rules:   rules,
			})
			if err != nil {
				return fmt.Errorf("initialize firewall: %w", err)
			}
			if fw != nil {
				proxyOpts = append(proxyOpts, proxy.WithFirewall(fw))
			}
		}

		// Initialize smart router
		if cfg.Routing.Enabled {
			tiers := make(map[string]router.TierConfig, len(cfg.Routing.Tiers))
			for name, t := range cfg.Routing.Tiers {
				tiers[name] = router.TierConfig{
					MaxMessageTokens: t.MaxMessageTokens,
					MaxMessages:      t.MaxMessages,
					KeywordsAbsent:   t.KeywordsAbsent,
				}
			}
			rt := router.New(router.Config{
				Enabled:  true,
				Tiers:    tiers,
				ModelMap: cfg.Routing.ModelMap,
			})
			if rt != nil {
				proxyOpts = append(proxyOpts, proxy.WithRouter(rt))
			}
		}

		// Create proxy
		p := proxy.New(cfg, st, proxyOpts...)

		// Set up HTTP handler (proxy + optional dashboard)
		var handler http.Handler = p
		if cfg.Dashboard.Enabled {
			mux := http.NewServeMux()
			dash := dashboard.New(cfg, st)
			dash.Register(mux)
			// Proxy handles all non-dashboard routes
			mux.Handle("/", p)
			handler = mux
		}

		addr := fmt.Sprintf(":%d", cfg.Port)
		srv := &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		// Handle graceful shutdown
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println()
			fmt.Println(ui.Dimf("Shutting down proxy server..."))
			srv.Close()
		}()

		// Startup banner
		fmt.Println()
		fmt.Println(ui.Boldf("  agix") + ui.Dimf(" - AI agent gateway"))
		fmt.Println()
		fmt.Printf("  %s  %s\n", ui.Dimf("Listening:"), ui.Greenf("http://localhost%s", addr))
		fmt.Printf("  %s  %s\n", ui.Dimf("Database: "), cfg.Database)
		fmt.Println()

		// Show configured providers
		fmt.Printf("  %s\n", ui.Dimf("Providers:"))
		if key, ok := cfg.Keys["openai"]; ok && key != "" {
			fmt.Printf("    %s  %s\n", ui.Greenf("openai"), ui.Dimf("gpt-*, o1-*, o3-*"))
		} else {
			fmt.Printf("    %s  %s\n", ui.Dimf("openai"), ui.Yellowf("not configured"))
		}
		if key, ok := cfg.Keys["anthropic"]; ok && key != "" {
			fmt.Printf("    %s  %s\n", ui.Greenf("anthropic"), ui.Dimf("claude-*"))
		} else {
			fmt.Printf("    %s  %s\n", ui.Dimf("anthropic"), ui.Yellowf("not configured"))
		}
		if key, ok := cfg.Keys["deepseek"]; ok && key != "" {
			fmt.Printf("    %s  %s\n", ui.Greenf("deepseek"), ui.Dimf("deepseek-*"))
		} else {
			fmt.Printf("    %s  %s\n", ui.Dimf("deepseek"), ui.Yellowf("not configured"))
		}
		fmt.Println()

		// Show how to connect
		fmt.Printf("  %s\n", ui.Dimf("Connect your agents:"))
		fmt.Printf("    %s\n", ui.Cyanf("OPENAI_BASE_URL=http://localhost%s/v1", addr))
		fmt.Println()

		// Show budget info
		if len(cfg.Budgets) > 0 {
			fmt.Printf("  %s %d agent(s) with budget limits\n", ui.Dimf("Budgets:"), len(cfg.Budgets))
			fmt.Println()
		}

		// Show MCP tools info
		if toolMgr != nil {
			fmt.Printf("  %s %d tool(s) from %d MCP server(s)\n",
				ui.Dimf("Tools:  "), toolMgr.ToolCount(), toolMgr.ServerCount())
			fmt.Printf("  %s %d\n",
				ui.Dimf("Max iterations:"), cfg.Tools.MaxIterations)
			fmt.Println()
		}

		// Show dashboard info
		if cfg.Dashboard.Enabled {
			fmt.Printf("  %s %s\n", ui.Dimf("Dashboard:"), ui.Cyanf("http://localhost%s/dashboard", addr))
			fmt.Println()
		}

		fmt.Println(ui.Dimf("  Press Ctrl+C to stop"))
		fmt.Println()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().IntVarP(&startPort, "port", "p", 0, "port to listen on (overrides config)")
}

func loadConfig() (*config.Config, string, error) {
	path := cfgFile
	if path == "" {
		var err error
		path, err = config.DefaultConfigPath()
		if err != nil {
			return nil, "", fmt.Errorf("determine config path: %w", err)
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("config not found at %s (run 'agix init' first)", path)
		}
		return nil, "", err
	}

	return cfg, path, nil
}
