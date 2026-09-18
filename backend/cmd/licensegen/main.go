// Command licensegen 提供离线商业许可证（ROADMAP P3 商业化）的密钥生成、签名签发与验证工具。
//
// 使用说明:
//   1. 生成密钥对:
//      licensegen keygen
//
//   2. 签发许可证:
//      licensegen sign -key-id lk1 -priv <base64_privkey> -edition enterprise -to "Customer Name" -devices 1000 -days 365 -features "scada,edge_ops"
//
//   3. 检验许可证:
//      licensegen verify -key-id lk1 -pub <base64_pubkey> -license <base64_license>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/license"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "keygen":
		handleKeygen()
	case "sign":
		handleSign(os.Args[2:])
	case "verify":
		handleVerify(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`AetherLink 离线商业许可证工具 (licensegen)

用法:
  licensegen <command> [arguments]

可用命令:
  keygen   生成新的 Ed25519 签名密钥对 (公钥 + 私钥)
  sign     签署并生成离线许可证材料
  verify   验证并解析已签发的许可证材料

运行 'licensegen <command> -h' 查看具体命令的参数说明。`)
}

func handleKeygen() {
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成密钥对失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== 新生成的 Ed25519 密钥对 ===")
	fmt.Printf("公钥 (用于服务端部署配置 license.public_keys):\n  %s\n\n", pub)
	fmt.Printf("私钥 (仅供官方签发机构妥善保管，切勿泄露):\n  %s\n", priv)
}

func handleSign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyID := fs.String("key-id", "", "密钥标识符 (例如 lk1) [必填]")
	privKey := fs.String("priv", "", "Base64 编码的 Ed25519 私钥 [必填]")
	edition := fs.String("edition", "enterprise", "商业版本 (community / professional / enterprise)")
	issuedTo := fs.String("to", "", "被授权方企业名称或合同号")
	devices := fs.Int64("devices", 0, "最大设备配额 (0 表示不限量)")
	tenants := fs.Int64("tenants", 0, "最大租户配额 (0 表示不限量)")
	days := fs.Int("days", 365, "许可证有效天数")
	features := fs.String("features", "", "启用的特性列表，逗号分隔 (例如 scada,edge_ops)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *keyID == "" || *privKey == "" {
		fmt.Fprintln(os.Stderr, "错误: -key-id 与 -priv 为必填参数")
		fs.Usage()
		os.Exit(1)
	}

	now := time.Now().UTC()
	var notAfter int64
	if *days > 0 {
		notAfter = now.Add(time.Duration(*days) * 24 * time.Hour).UnixMilli()
	}

	var featureList []string
	if *features != "" {
		for _, f := range strings.Split(*features, ",") {
			trimmed := strings.TrimSpace(f)
			if trimmed != "" {
				featureList = append(featureList, trimmed)
			}
		}
	}

	doc := &license.Document{
		Edition:    strings.TrimSpace(*edition),
		IssuedTo:   strings.TrimSpace(*issuedTo),
		Features:   featureList,
		MaxDevices: *devices,
		MaxTenants: *tenants,
		NotBefore:  now.Add(-5 * time.Minute).UnixMilli(), // 预留 5 分钟时钟漂移
		NotAfter:   notAfter,
		IssuedAt:   now.UnixMilli(),
	}

	material, err := license.SignDocumentWithBase64Key(doc, *keyID, *privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "签发许可证失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== 许可证签发成功 ===")
	fmt.Printf("版本: %s\n", doc.Edition)
	fmt.Printf("客户: %s\n", doc.IssuedTo)
	fmt.Printf("设备上限: %d\n", doc.MaxDevices)
	fmt.Printf("租户上限: %d\n", doc.MaxTenants)
	if len(doc.Features) > 0 {
		fmt.Printf("授权特性: %s\n", strings.Join(doc.Features, ", "))
	}
	if notAfter > 0 {
		fmt.Printf("有效期至: %s\n", time.UnixMilli(notAfter).Format(time.RFC3339))
	} else {
		fmt.Println("有效期至: 永久有效")
	}
	fmt.Println("\n许可证材料 (配置至 conf.yml 的 license.material):")
	fmt.Println(material)
}

func handleVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	keyID := fs.String("key-id", "", "密钥标识符 [必填]")
	pubKey := fs.String("pub", "", "Base64 编码的公钥 [必填]")
	licMaterial := fs.String("license", "", "Base64 编码的许可证材料 [必填]")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *keyID == "" || *pubKey == "" || *licMaterial == "" {
		fmt.Fprintln(os.Stderr, "错误: -key-id, -pub 与 -license 为必填参数")
		fs.Usage()
		os.Exit(1)
	}

	verifier, err := license.NewVerifier(map[string]string{*keyID: *pubKey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化验证器失败: %v\n", err)
		os.Exit(1)
	}

	doc, fingerprint, err := verifier.Parse(*licMaterial, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "许可证校验失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== 许可证校验通过 ===")
	fmt.Printf("数字指纹 (SHA-256): %s\n", fingerprint)
	fmt.Printf("版本: %s\n", doc.Edition)
	fmt.Printf("客户: %s\n", doc.IssuedTo)
	fmt.Printf("设备上限: %d\n", doc.MaxDevices)
	fmt.Printf("租户上限: %d\n", doc.MaxTenants)
	if len(doc.Features) > 0 {
		fmt.Printf("授权特性: %s\n", strings.Join(doc.Features, ", "))
	}
	if doc.NotAfter > 0 {
		fmt.Printf("到期时间: %s\n", time.UnixMilli(doc.NotAfter).Format(time.RFC3339))
	}
}
