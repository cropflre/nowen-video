from pathlib import Path

p = Path('config.example.yaml')
s = p.read_text(encoding='utf-8')
old = '''secrets:
  jwt_secret: please-change-this-secret-key
  tmdb_api_key: ""'''
new = '''secrets:
  jwt_secret: please-change-this-secret-key
  tmdb_api_key: ""

  # API / 图片反向代理 Base URL，不是 curl -x 类型的 forward proxy。
  # 程序会分别自动拼接 /3/... 与 /t/p/...
  tmdb_api_proxy: ""
  tmdb_image_proxy: ""

  # HTTP/SOCKS 网络出口代理，适合 Clash / v2ray / Shadowsocks / Karing。
  # 支持：http://、https://、socks5://、socks5h://
  # 示例：http://192.168.1.11:7890 或 socks5://127.0.0.1:7891
  tmdb_network_proxy: ""'''
assert s.count(old) == 1
p.write_text(s.replace(old, new, 1), encoding='utf-8')
