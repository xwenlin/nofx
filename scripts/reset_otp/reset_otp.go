package main

import (
	"flag"
	"log"
	"nofx/auth"
	"nofx/config"
)

func main() {
	email := flag.String("email", "", "用户邮箱（用于查找用户）")
	userID := flag.String("user-id", "", "用户ID（用于查找用户，优先级高于邮箱）")
	dbPath := flag.String("db", "config.db", "数据库文件路径")
	flag.Parse()

	// 验证参数
	if *userID == "" && *email == "" {
		log.Fatal("❌ 错误：必须提供 -user-id 或 -email 参数之一")
	}

	// 打开数据库
	log.Printf("📂 打开数据库: %s", *dbPath)
	database, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer database.Close()

	// 查找用户
	var targetUser *config.User
	if *userID != "" {
		// 优先使用userID
		targetUser, err = database.GetUserByID(*userID)
		if err != nil {
			log.Fatalf("❌ 用户不存在 (ID: %s): %v", *userID, err)
		}
	} else {
		// 使用邮箱查找
		targetUser, err = database.GetUserByEmail(*email)
		if err != nil {
			log.Fatalf("❌ 用户不存在 (邮箱: %s): %v", *email, err)
		}
	}

	log.Printf("✓ 找到用户:")
	log.Printf("   用户ID: %s", targetUser.ID)
	log.Printf("   邮箱: %s", targetUser.Email)

	// 重置OTP密钥
	log.Printf("\n🔄 正在重置OTP密钥...")
	newOTPSecret, err := database.ResetUserOTP(targetUser.ID)
	if err != nil {
		log.Fatalf("❌ 重置OTP密钥失败: %v", err)
	}

	log.Printf("✅ OTP密钥重置成功！")

	// 生成OTP二维码URL
	qrCodeURL := auth.GetOTPQRCodeURL(newOTPSecret, targetUser.Email)
	log.Printf("\n📱 新的OTP设置信息:")
	log.Printf("   二维码URL: %s", qrCodeURL)
	log.Printf("   密钥: %s", newOTPSecret)
	log.Printf("\n💡 下一步操作:")
	log.Printf("   1. 使用邮箱和密码登录系统")
	log.Printf("   2. 登录后系统会自动显示OTP设置页面")
	log.Printf("   3. 使用Google Authenticator扫描二维码或手动输入密钥")
	log.Printf("   4. 完成OTP设置后即可正常使用")
	log.Printf("\n⚠️  注意: 请妥善保管新的OTP密钥，旧手机的OTP验证码将不再有效")
}
