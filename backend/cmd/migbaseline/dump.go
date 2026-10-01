package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"aetherlink-iot/backend/initialize"
)

// findPGDump 依次查找 PG_BIN、PATH、C:/Program Files/PostgreSQL/<最高版本>/bin。
func findPGDump() string {
	exe := "pg_dump"
	if bin := os.Getenv("PG_BIN"); bin != "" {
		for _, name := range []string{"pg_dump", "pg_dump.exe"} {
			if p := filepath.Join(bin, name); fileExists(p) {
				return p
			}
		}
	}
	if p, err := exec.LookPath(exe); err == nil {
		return p
	}
	matches, _ := filepath.Glob("C:/Program Files/PostgreSQL/*/bin/pg_dump.exe")
	best, bestVer := "", -1
	for _, m := range matches {
		ver, _ := strconv.Atoi(filepath.Base(filepath.Dir(filepath.Dir(m))))
		if ver > bestVer {
			best, bestVer = m, ver
		}
	}
	if best == "" {
		fail(2, "找不到 pg_dump（设置 PG_BIN 或加入 PATH）")
	}
	return best
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// withDB 把 keyword=value 连接串中的 dbname 替换为 name（没有则追加）。
func withDB(dsn, name string) string {
	fields := strings.Fields(dsn)
	out := make([]string, 0, len(fields)+1)
	for _, f := range fields {
		if !strings.HasPrefix(f, "dbname=") {
			out = append(out, f)
		}
	}
	return strings.Join(append(out, "dbname="+name), " ")
}

func pgDump(bin, dsn string, args ...string) []byte {
	cmd := exec.Command(bin, append([]string{"--dbname=" + dsn, "--no-owner", "--no-privileges",
		"--exclude-table=public.sys_version"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		fail(2, "pg_dump 失败：%v\n%s", err, stderr.String())
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		fmt.Fprintln(os.Stderr, "pg_dump:", s)
	}
	return bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
}

// 会话级 SET 与 search_path 置空都会泄漏到同一事务里后续的增量迁移，必须剔除。
var (
	sessionSetRe = regexp.MustCompile(`^SET [a-z_]+ = .*;$`)
	searchPathRe = regexp.MustCompile(`^SELECT pg_catalog\.set_config\('search_path', '', false\);$`)
)

// writeBaseline 导出 db 全量（schema + 数据 + setval）并后处理为可在单事务内 tx.Exec 的基线文件。
func writeBaseline(bin, dsn, outPath string, n int, pgVersion string) {
	raw := pgDump(bin, dsn, "--column-inserts", "--rows-per-insert=200")
	sha, err := initialize.BaselineSourceSHA256("sql", n)
	if err != nil {
		fail(2, "计算源 sha256 失败：%v", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "-- sql/baseline/%d.sql —— 由 cmd/migbaseline 生成，禁止手改（改了任何 sql/1..%d.sql 必须重新生成）。\n", n, n)
	fmt.Fprintf(&b, "-- generator: cd backend && go run ./cmd/migbaseline -dsn-admin <admin-dsn> -verify\n")
	fmt.Fprintf(&b, "-- postgres: %s（AETHERLINK_TIMESCALE_MODE=off；TimescaleDB 安装不使用本基线）\n", pgVersion)
	fmt.Fprintf(&b, "%s1..%d\n", initialize.BaselineHeaderRange, n)
	fmt.Fprintf(&b, "%s%s\n", initialize.BaselineHeaderSHA, sha)
	b.WriteString("-- 种子行的 now() 时间戳为生成时刻，而非安装时刻。\n\n")
	b.WriteString("SET LOCAL check_function_bodies = false;\nSET LOCAL statement_timeout = 0;\nSET LOCAL lock_timeout = 0;\n\n")
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case sessionSetRe.MatchString(line), searchPathRe.MatchString(line):
			continue
		case strings.HasPrefix(line, `\restrict`), strings.HasPrefix(line, `\unrestrict`):
			continue
		case strings.HasPrefix(line, `\`), strings.HasPrefix(line, "COPY "):
			fail(2, "dump 中出现 psql 元命令或 COPY，无法经 tx.Exec 执行：%q", line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("SET LOCAL check_function_bodies = true;\n")
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		fail(2, "写入基线失败：%v", err)
	}
}

// varchar IN(...) 经 pg_dump 再装载后会被反解析为 ANY(ARRAY[('x'::varchar)::text,…])，语义不变。
var castRe = regexp.MustCompile(`::(character varying(\(\d+\))?|text\[\]|text)`)

func normalize(dump []byte) []string {
	lines := strings.Split(string(dump), "\n")
	out := lines[:0]
	for _, l := range lines {
		if strings.HasPrefix(l, "-- ") || strings.HasPrefix(l, `\restrict`) || strings.HasPrefix(l, `\unrestrict`) {
			continue // 注释里含 dump 时间/库名
		}
		if strings.Contains(l, "ARRAY[") {
			l = strings.NewReplacer("(", "", ")", "", " ", "").Replace(castRe.ReplaceAllString(l, ""))
		}
		out = append(out, l)
	}
	return out
}

// compareDumps 分别比对 schema（有序）与数据（单行 INSERT 多重集），返回差异行数并打印前若干行。
func compareDumps(bin, dsnA, dsnB, work string) int {
	total := 0
	for _, part := range []struct {
		name    string
		args    []string
		ordered bool
	}{
		{"schema", []string{"--schema-only"}, true},
		{"data", []string{"--data-only", "--column-inserts"}, false},
	} {
		a := normalize(pgDump(bin, dsnA, part.args...))
		b := normalize(pgDump(bin, dsnB, part.args...))
		os.WriteFile(filepath.Join(work, "a_"+part.name+".sql"), []byte(strings.Join(a, "\n")), 0o644)
		os.WriteFile(filepath.Join(work, "b_"+part.name+".sql"), []byte(strings.Join(b, "\n")), 0o644)
		onlyA, onlyB := multisetDiff(a, b)
		n := len(onlyA) + len(onlyB)
		if n == 0 && part.ordered && strings.Join(a, "\n") != strings.Join(b, "\n") {
			n = 1
			fmt.Printf("%s：行集合相同但顺序不同\n", part.name)
		}
		fmt.Printf("%s：A %d 行，B %d 行，差异 %d 行\n", part.name, len(a), len(b), n)
		for i, l := range onlyA {
			if i < 20 {
				fmt.Println("  - A only:", l)
			}
		}
		for i, l := range onlyB {
			if i < 20 {
				fmt.Println("  + B only:", l)
			}
		}
		total += n
	}
	return total
}

func multisetDiff(a, b []string) (onlyA, onlyB []string) {
	count := map[string]int{}
	for _, l := range a {
		count[l]++
	}
	for _, l := range b {
		count[l]--
	}
	for l, c := range count {
		for ; c > 0; c-- {
			onlyA = append(onlyA, l)
		}
		for ; c < 0; c++ {
			onlyB = append(onlyB, l)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}
