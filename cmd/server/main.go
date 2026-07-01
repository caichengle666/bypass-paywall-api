package main

import (
  "flag"
  "log"
  "os"
  "github.com/user/wsj-unlock-go-v2/internal/api"
  "github.com/user/wsj-unlock-go-v2/internal/browser"
  "github.com/user/wsj-unlock-go-v2/internal/config"
)

func main() {
  addr := flag.String("addr", ":8080", "listen")
  configPath := flag.String("config", "config_sites.json", "BPC config")
  maxB := flag.Int("browsers", 2, "max Chrome instances")
  chromePath := flag.String("chrome", "", "Chrome path")
  proxy := flag.String("proxy", "", "SOCKS5 proxy")
  apiKey := flag.String("api-key", "", "API key for auth (or BPC_API_KEY env)")
  flag.Parse()

  key := *apiKey
  if key == "" { key = os.Getenv("BPC_API_KEY") }

  if key != "" { log.Printf("auth enabled (api-key provided)") }

  cfg := config.NewSitesConfig()
  if err := cfg.LoadFromJSON(*configPath); err != nil { log.Fatalf("config: %v", err) }
  log.Printf("loaded %d site rules", cfg.Count())

  pool := browser.NewPool(*maxB)
  if *chromePath != "" { pool.SetChromePath(*chromePath) }
  if *proxy != "" { pool.SetProxy(*proxy) }
  if err := pool.Start(); err != nil { log.Fatalf("pool: %v", err) }
  defer pool.Shutdown()

  srv := api.NewServer(pool, cfg)
  srv.SetAPIKey(key)
  log.Fatalf("serve: %v", srv.Serve(*addr))
}
