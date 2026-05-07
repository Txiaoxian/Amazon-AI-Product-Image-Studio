# Amazon AI Product Image Studio

React + TypeScript + Vite 应用，用于亚马逊卖家的 AI 产品图片生成、参考图编辑、历史管理和本地保存。

## 技术栈

- React 19 + TypeScript + Vite
- Tailwind CSS
- Dexie.js / IndexedDB
- localStorage
- Nginx 静态部署

## 本地开发

```bash
npm install
npm run dev
```

手机在同一局域网访问开发服务：

```bash
npm run dev:host
```

## 检查命令

```bash
npm run lint
npm run type-check
npm run test
npm run build
```

## Docker 静态部署

```bash
docker build -t amazon-ai-product-image-studio .
docker run --rm -p 8080:80 amazon-ai-product-image-studio
```

访问 `http://localhost:8080`。

## 在 Mac M 系列上构建 x86 Docker 包

部署机器是 x86_64 / amd64 时，在 Mac mini M4 上使用 Buildx 指定 `linux/amd64`：

```bash
npm run docker:package:amd64
```

生成文件：`amazon-ai-product-image-studio-linux-amd64.tar`

如果 Docker Hub 访问受限，可以使用国内镜像源构建：

```bash
npm run docker:package:amd64:mirror
```

拷贝到 x86 部署机器后加载并运行：

```bash
docker load -i amazon-ai-product-image-studio-linux-amd64.tar
docker run -d --name amazon-ai-product-image-studio -p 8080:80 amazon-ai-product-image-studio:linux-amd64
```

## 安全说明

OpenAI 与 Gemini API Key 仅保存在当前浏览器的 `localStorage` 中，不会提交到项目文件。二号中转站默认 API URL 为 `https://api.flymux.com`，请求默认通过同源 `/relay2` 代理转发以规避浏览器 CORS 限制；这不会隐藏 API Key，部署该应用的服务器仍可看到转发请求。请勿在公共电脑或不可信设备上使用。
