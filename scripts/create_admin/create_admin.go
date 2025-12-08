package main

import (
	"flag"
	"fmt"
	"log"
	"nofx/auth"
	"nofx/config"
)

func main() {
	email := flag.String("email", "admin@example.com", "管理员邮箱")
	password := flag.String("password", "", "管理员密码（必填）")
	dbPath := flag.String("db", "config.db", "数据库文件路径")
	flag.Parse()

	if *password == "" {
		log.Fatal("❌ 错误：必须提供密码（使用 -password 参数）")
	}

	// 打开数据库
	log.Printf("📂 打开数据库: %s", *dbPath)
	database, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer database.Close()

	// 检查用户是否已存在
	existingUser, err := database.GetUserByEmail(*email)
	if err == nil {
		log.Fatalf("❌ 用户已存在: %s (ID: %s)", *email, existingUser.ID)
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("❌ 密码处理失败: %v", err)
	}

	// 生成OTP密钥（使用与用户注册相同的函数）
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		log.Fatalf("❌ OTP密钥生成失败: %v", err)
	}

	// 创建用户ID（使用邮箱作为基础）
	userID := "admin"
	if *email != "admin@example.com" {
		// 如果使用自定义邮箱，使用邮箱作为ID的一部分
		userID = fmt.Sprintf("admin_%s", *email)
	}

	// 创建用户
	user := &config.User{
		ID:           userID,
		Email:        *email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false, // 设置为false，首次登录时通过Web界面完成OTP设置
	}

	err = database.CreateUser(user)
	if err != nil {
		log.Fatalf("❌ 创建用户失败: %v", err)
	}

	log.Printf("✅ 管理员账号创建成功！")
	log.Printf("   用户ID: %s", userID)
	log.Printf("   邮箱: %s", *email)

	// 生成OTP二维码URL
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, *email)
	log.Printf("\n📱 OTP设置信息（首次登录时会自动显示）:")
	log.Printf("   二维码URL: %s", qrCodeURL)
	log.Printf("   密钥: %s", otpSecret)
	log.Printf("\n💡 登录步骤:")
	log.Printf("   1. 使用邮箱和密码登录系统")
	log.Printf("   2. 登录后系统会自动显示OTP设置页面")
	log.Printf("   3. 使用Google Authenticator扫描二维码或手动输入密钥")
	log.Printf("   4. 完成OTP设置后即可正常使用")
	log.Printf("\n⚠️  注意: 请妥善保管OTP密钥，丢失后需要重新设置")
}
