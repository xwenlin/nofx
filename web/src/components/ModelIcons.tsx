// 获取图标路径的工具函数
// BASE_URL 是 Vite 内置环境变量，自动从 vite.config.ts 的 base 配置中获取
// 例如 base: '/nofx/' 时，BASE_URL 的值就是 '/nofx/'
export const getIconPath = (iconName: string): string => {
  const baseUrl = import.meta.env.BASE_URL || '/';
  // BASE_URL 通常是 '/nofx/' 这样的格式（以 / 开头和结尾）
  // 直接拼接即可，不需要移除末尾斜杠
  return `${baseUrl}icons/${iconName}`;
};

interface IconProps {
  width?: number;
  height?: number;
  className?: string;
}

// 获取AI模型图标的函数
export const getModelIcon = (modelType: string, props: IconProps = {}) => {
  // 支持完整ID或类型名
  const type = modelType.includes('_') ? modelType.split('_').pop() : modelType;

  let iconPath: string | null = null;

  switch (type) {
    case 'deepseek':
      iconPath = getIconPath('deepseek.svg');
      break;
    case 'qwen':
      iconPath = getIconPath('qwen.svg');
      break;
    default:
      return null;
  }

  return (
    <img
      src={iconPath}
      alt={`${type} icon`}
      width={props.width || 24}
      height={props.height || 24}
      className={props.className}
      style={{ borderRadius: '50%' }}
    />
  );
};