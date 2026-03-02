# 💫 赛博拍屏

<div align="center">
<img height="256" alt="image" src="https://github.com/user-attachments/assets/dceb90f3-9d58-4426-9335-7533efdc1027" />
<img height="256" alt="image" src="https://github.com/user-attachments/assets/35780373-b460-44c0-be21-8260b76be8a2" />
<img height="256" alt="image" src="https://github.com/user-attachments/assets/9d035f0b-f4b3-44fa-b093-5b941fe1cf81" />

<p>
</div>

> ~~无内鬼，来点内部消息~~ 🚨 红线警告

本项目通过 LLM 分析图片并生成 HTML 渲染，以达到去除可见水印和[盲水印](https://www.volcengine.com/docs/508/124670?lang=zh)的功能。

单个图片分享支持独立密码，服务端不保存原图只保留 32*32px 缩略图，以降低服务端被技术接管后的潜在安全风险。

单个图片预计 <5000 token 消耗，过大、过复杂、无结构化图片会影响任务处理，大家可以针对自己的场景自行优化 prmopt。

# 💫 no-track-screenshot

Watermark removal through AI-powered HTML reconstruction.

## Purpose

Screenshots shared online often contain visible watermarks (text overlays, logos) and invisible watermarks (steganographic data embedded in pixels that can trace the image back to the original viewer). This tool bypasses both by never sharing the original image — instead, it uses a vision AI to **reconstruct the content as a clean HTML page**.

## How It Works

1. **Upload** — You upload a screenshot through the web UI. The image is held in memory only; it is never written to disk.
2. **Thumbnail** — A 32×32 pixel JPEG preview is generated from the image and stored in SQLite as a reference.
3. **AI reconstruction** — The full image is base64-encoded and sent to an OpenAI-compatible vision API along with a prompt instructing the model to reproduce the text, layout, colors, and overall style as an HTML page. Avatars, logos, and other identifying images are replaced with placeholders.
4. **Serve** — The generated HTML is stored in SQLite and served at a unique task URL. The original image is discarded after the API call.
5. **Access control** — Each task URL can be protected with the global password or a per-task password that can be shared independently.

## Why This Works

- **Visible watermarks** are ignored by the AI — it reconstructs the semantic content, not the pixel data.
- **Invisible watermarks** cannot survive reconstruction because the output is synthesized HTML, not a modified copy of the original image.
- The original image never leaves your server in a shareable form.

## Stack

- **Backend**: Go 1.22, standard `net/http`
- **Database**: SQLite (via `go-sqlite3`)
- **AI**: Any OpenAI-compatible vision API (OpenAI, Gemini, etc.)
- **Config**: YAML

## Configuration

Copy `config.yaml` and fill in your values:

```yaml
server:
  port: 8080

password: "your-global-password"

ai:
  endpoint: "https://api.openai.com/v1/chat/completions"
  key: "sk-..."
  model: "gpt-4o"
  mock: false   # set true to skip AI calls and return example HTML
```

The AI prompt is read from `prompt.txt` at startup and can be edited without recompiling.

## Running

```bash
go build -o no-track-screenshot .
./no-track-screenshot              # uses config.yaml by default
./no-track-screenshot myconfig.yaml
```

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=CyrusF/no-track-screenshot&type=timeline&legend=top-left)](https://www.star-history.com/#CyrusF/no-track-screenshot&type=timeline&legend=top-left)
