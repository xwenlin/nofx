const path = require('path');
const fs = require('fs');

// 加载 .env 文件
function loadEnvFile() {
  const envPath = path.join(__dirname, '.env');
  const env = { NODE_ENV: 'production' };

  if (fs.existsSync(envPath)) {
    const envContent = fs.readFileSync(envPath, 'utf8');
    const lines = envContent.split('\n');

    for (const line of lines) {
      const trimmedLine = line.trim();
      // 跳过空行和注释
      if (!trimmedLine || trimmedLine.startsWith('#')) {
        continue;
      }

      // 解析 KEY=VALUE 格式
      const match = trimmedLine.match(/^([^=]+)=(.*)$/);
      if (match) {
        const key = match[1].trim();
        let value = match[2].trim();
        // 移除引号（如果有）
        if ((value.startsWith('"') && value.endsWith('"')) ||
          (value.startsWith("'") && value.endsWith("'"))) {
          value = value.slice(1, -1);
        }
        env[key] = value;
      }
    }
  }

  return env;
}

module.exports = {
  apps: [
    {
      name: 'nofx-backend',
      script: './nofx',
      cwd: __dirname, // 使用当前目录（配置文件所在目录）
      interpreter: 'none', // 不使用解释器，直接执行二进制文件
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '256M',
      env: loadEnvFile(),
      error_file: './logs/backend-error.log',
      out_file: './logs/backend-out.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    }
  ]
};
