package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

var groupNumberPattern = regexp.MustCompile(`^\d{5,12}$`)

// runGroupCLI 处理 `lol-civil-war group <add|remove|list>`，用于在线维护白名单。
func runGroupCLI(args []string) int {
	if len(args) == 0 {
		printGroupUsage()
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		return 1
	}
	st, err := store.Open(cfg.DBDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		return 1
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch args[0] {
	case "add", "allow":
		if len(args) < 2 {
			printGroupUsage()
			return 2
		}
		number := strings.TrimSpace(args[1])
		if !groupNumberPattern.MatchString(number) {
			fmt.Fprintln(os.Stderr, "群号必须是 5-12 位数字")
			return 2
		}
		note := strings.TrimSpace(strings.Join(args[2:], " "))
		if err := st.AddAllowedGroup(ctx, number, note); err != nil {
			fmt.Fprintf(os.Stderr, "添加白名单失败: %v\n", err)
			return 1
		}
		fmt.Printf("已添加白名单群 %s（立即生效，无需重启）\n", number)

	case "remove", "rm", "deny":
		if len(args) < 2 {
			printGroupUsage()
			return 2
		}
		number := strings.TrimSpace(args[1])
		removed, err := st.RemoveAllowedGroup(ctx, number)
		if err != nil {
			fmt.Fprintf(os.Stderr, "删除白名单失败: %v\n", err)
			return 1
		}
		if !removed {
			fmt.Printf("群 %s 不在数据库白名单中\n", number)
			return 0
		}
		fmt.Printf("已移除白名单群 %s（立即生效）\n", number)
		for _, envNumber := range cfg.AllowedGroups {
			if envNumber == number {
				fmt.Println("注意：该群仍在环境变量 LCW_ALLOWED_GROUPS 中，仍然可以访问")
				break
			}
		}

	case "list", "ls":
		numbers, err := st.AllowedGroupNumbers(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取白名单失败: %v\n", err)
			return 1
		}
		fmt.Println("数据库白名单：")
		if len(numbers) == 0 {
			fmt.Println("  (空)")
		}
		for _, number := range numbers {
			group, err := st.GroupByNumber(ctx, number)
			if err != nil {
				fmt.Printf("  %s  (尚未创建)\n", number)
				continue
			}
			name := ""
			if group.Name != nil {
				name = *group.Name
			}
			fmt.Printf("  %s  key=%s  name=%s\n", number, group.PublicKey, name)
		}
		if len(cfg.AllowedGroups) > 0 {
			fmt.Println("环境变量 LCW_ALLOWED_GROUPS（始终允许）：")
			for _, number := range cfg.AllowedGroups {
				fmt.Printf("  %s\n", number)
			}
		}

	default:
		printGroupUsage()
		return 2
	}
	return 0
}

func printGroupUsage() {
	fmt.Fprintln(os.Stderr, `用法：
  lol-civil-war group add <群号> [备注]   # 添加白名单（别名 allow）
  lol-civil-war group remove <群号>      # 移除白名单（别名 rm/deny）
  lol-civil-war group list               # 查看数据库白名单（别名 ls）

Docker 部署时：
  docker compose exec app /app/lol-civil-war group add 123456789`)
}
