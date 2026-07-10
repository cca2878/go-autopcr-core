// Command datagen 生成派生代码与数据（母数据库、协议 DTO 等）。
//
// 已实现子命令：
//   - extract    <in.unity3d> <out.db>                  仅提取内嵌 SQLite（未反混淆）
//   - masterdata <in.unity3d> <rainbow.json> <out.db>   提取 + 反混淆 → 落盘干净母数据库
//
// 反混淆是一次性离线步骤：产出的干净库由运行时只读消费，不再每次反混淆。
// 协议 DTO 生成等将在后续里程碑加入。
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/asset"
	"github.com/cca2878/go-autopcr/internal/client/unityfs"
	_ "modernc.org/sqlite"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	switch {
	case len(args) == 3 && args[0] == "extract":
		if err := extractMasterdata(args[1], args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "datagen extract 失败:", err)
			return 1
		}
		return 0
	case len(args) == 4 && args[0] == "masterdata":
		if err := buildMasterdata(args[1], args[2], args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "datagen masterdata 失败:", err)
			return 1
		}
		return 0
	case len(args) == 4 && args[0] == "fetch":
		if err := fetchMasterdata(args[1], args[2], args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "datagen fetch 失败:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprint(os.Stderr, `datagen — go-autopcr 数据/代码生成器

用法:
  datagen extract    <in.unity3d> <out.db>                  仅提取内嵌 SQLite（未反混淆）
  datagen masterdata <in.unity3d> <rainbow.json> <out.db>   提取 + 反混淆 → 干净母数据库
  datagen fetch      <ver> <rainbow.json> <cacheDir>        在线下载指定版本 + 反混淆 → 缓存干净库
`)
		return 2
	}
}

// fetchMasterdata 在线下载指定版本的 masterdata 并产出缓存的干净库（对应原型 fetch）。
func fetchMasterdata(verStr, rainbowPath, cacheDir string) error {
	ver, err := strconv.Atoi(verStr)
	if err != nil {
		return fmt.Errorf("版本号无效 %q: %w", verStr, err)
	}
	rainbowData, err := os.ReadFile(rainbowPath)
	if err != nil {
		return err
	}
	rainbow, err := masterdata.ParseRainbow(rainbowData)
	if err != nil {
		return err
	}

	mgr := masterdata.NewManager(cacheDir, rainbow, asset.NewSource())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	path, err := mgr.EnsureDB(ctx, ver)
	if err != nil {
		return err
	}
	fmt.Printf("干净库就绪: %s（耗时 %s）\n", path, time.Since(start).Round(time.Millisecond))
	return nil
}

func extractMasterdata(inPath, outPath string) error {
	start := time.Now()
	db, err := extractSQLite(inPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, db, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(db)
	fmt.Printf("提取完成: %d 字节 -> %s\nsha256 = %x\n耗时 %s\n", len(db), outPath, sum, time.Since(start).Round(time.Millisecond))
	return nil
}

func buildMasterdata(unity3dPath, rainbowPath, outPath string) error {
	// 1) 提取内嵌 SQLite 并落盘
	tExtract := time.Now()
	sqliteBytes, err := extractSQLite(unity3dPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, sqliteBytes, 0o644); err != nil {
		return err
	}
	fmt.Printf("提取: %d 字节，耗时 %s\n", len(sqliteBytes), time.Since(tExtract).Round(time.Millisecond))

	// 2) 载入 rainbow 表
	rainbowData, err := os.ReadFile(rainbowPath)
	if err != nil {
		return err
	}
	rainbow, err := masterdata.ParseRainbow(rainbowData)
	if err != nil {
		return err
	}

	// 3) 原地反混淆（含 ANALYZE 重建统计）
	db, err := sql.Open("sqlite", outPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	tUnhash := time.Now()
	n, err := masterdata.Unhash(db, rainbow)
	if err != nil {
		return err
	}
	fmt.Printf("反混淆: 还原 %d 张表，耗时 %s\n", n, time.Since(tUnhash).Round(time.Millisecond))
	fmt.Printf("干净库就绪 -> %s（%d 字节）\n", outPath, len(sqliteBytes))
	return nil
}

func extractSQLite(inPath string) ([]byte, error) {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	return unityfs.ExtractSQLite(raw)
}
