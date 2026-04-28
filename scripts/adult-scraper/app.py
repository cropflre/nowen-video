# nowen-video 番号刮削 Python 微服务
# 用于处理 Cloudflare 等强反爬场景，作为 Go 原生爬虫的 fallback
#
# 部署方式：
#   pip install -r requirements.txt
#   python app.py
#
# 默认监听 http://localhost:5000
# 在 nowen-video 配置中设置：
#   adult_scraper:
#     python_service_url: "http://localhost:5000"

from flask import Flask, request, jsonify
import requests
from bs4 import BeautifulSoup
import re
import time
import random
import os
import logging

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# API Key 认证（可选）
API_KEY = os.environ.get("SCRAPER_API_KEY", "")

# ==================== 请求工具 ====================

# 随机 User-Agent 池
USER_AGENTS = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
]

def get_headers():
    """生成随机浏览器请求头"""
    return {
        "User-Agent": random.choice(USER_AGENTS),
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
        "Accept-Language": "zh-CN,zh;q=0.9,ja;q=0.8,en;q=0.7",
        "Accept-Encoding": "gzip, deflate, br",
        "Connection": "keep-alive",
        "Upgrade-Insecure-Requests": "1",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "none",
        "Sec-Fetch-User": "?1",
    }

session = requests.Session()

# ==================== JavBus 刮削 ====================

def scrape_javbus(code):
    """从 JavBus 刮削番号元数据"""
    base_url = os.environ.get("JAVBUS_URL", "https://www.javbus.com")
    url = f"{base_url}/{code}"
    logger.info(f"JavBus 刮削: {url}")

    try:
        resp = session.get(url, headers=get_headers(), timeout=15)
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
    except Exception as e:
        logger.warning(f"JavBus 请求失败: {e}")
        return None

    soup = BeautifulSoup(resp.text, "html.parser")
    meta = {"code": code, "source": "javbus"}

    # 标题
    h3 = soup.find("h3")
    if h3:
        title = h3.get_text(strip=True)
        # 去掉番号前缀
        title = re.sub(rf"^{re.escape(code)}\s*", "", title)
        meta["title"] = title.strip()

    # 封面
    big_img = soup.find("a", class_="bigImage")
    if big_img:
        img = big_img.find("img")
        if img and img.get("src"):
            meta["cover"] = img["src"]

    # 信息面板
    info = soup.find("div", class_="col-md-3 info")
    if info:
        for p in info.find_all("p"):
            text = p.get_text()
            # 制作商
            if "製作商" in text:
                a = p.find("a")
                if a:
                    meta["studio"] = a.get_text(strip=True)
            # 發行商
            elif "發行商" in text:
                a = p.find("a")
                if a:
                    meta["label"] = a.get_text(strip=True)
            # 系列
            elif "系列" in text:
                a = p.find("a")
                if a:
                    meta["series"] = a.get_text(strip=True)
            # 日期
            elif "發行日期" in text:
                date_match = re.search(r"(\d{4}-\d{2}-\d{2})", text)
                if date_match:
                    meta["release_date"] = date_match.group(1)
            # 時長
            elif "長度" in text:
                dur_match = re.search(r"(\d+)分鐘", text)
                if dur_match:
                    meta["duration"] = int(dur_match.group(1))

    # 演員
    actresses = []
    for avatar in soup.find_all("a", class_="avatar-box"):
        span = avatar.find("span")
        if span:
            actresses.append(span.get_text(strip=True))
    if actresses:
        meta["actresses"] = actresses

    # 類別
    genres = []
    genre_section = soup.find("span", class_="genre")
    if genre_section:
        parent = genre_section.parent
        if parent:
            for a in parent.find_all("a"):
                genres.append(a.get_text(strip=True))
    if genres:
        meta["genres"] = genres

    return meta if meta.get("title") else None


# ==================== JavDB 刮削 ====================

def scrape_javdb(code):
    """从 JavDB 刮削番号元数据"""
    base_url = os.environ.get("JAVDB_URL", "https://javdb.com")
    search_url = f"{base_url}/search?q={code}&f=all"
    logger.info(f"JavDB 搜索: {search_url}")

    try:
        resp = session.get(search_url, headers=get_headers(), timeout=15)
        resp.raise_for_status()
    except Exception as e:
        logger.warning(f"JavDB 搜索请求失败: {e}")
        return None

    soup = BeautifulSoup(resp.text, "html.parser")

    # 查找匹配的搜索结果
    detail_url = None
    for item in soup.find_all("a", class_="box"):
        item_text = item.get_text().upper()
        if code.upper() in item_text:
            href = item.get("href")
            if href:
                detail_url = f"{base_url}{href}" if href.startswith("/") else href
                break

    if not detail_url:
        # 取第一个结果
        first = soup.find("a", class_="box")
        if first and first.get("href"):
            href = first["href"]
            detail_url = f"{base_url}{href}" if href.startswith("/") else href

    if not detail_url:
        return None

    # 访问详情页
    time.sleep(random.uniform(1, 2))
    logger.info(f"JavDB 详情: {detail_url}")

    try:
        resp2 = session.get(detail_url, headers=get_headers(), timeout=15)
        resp2.raise_for_status()
    except Exception as e:
        logger.warning(f"JavDB 详情请求失败: {e}")
        return None

    soup2 = BeautifulSoup(resp2.text, "html.parser")
    meta = {"code": code, "source": "javdb"}

    # 标题
    title_el = soup2.find("h2", class_="title")
    if title_el:
        strong = title_el.find("strong")
        if strong:
            title = strong.get_text(strip=True)
            title = re.sub(rf"^{re.escape(code)}\s*", "", title)
            meta["title"] = title.strip()

    # 封面
    cover_img = soup2.find("img", class_="video-cover")
    if cover_img and cover_img.get("src"):
        meta["cover"] = cover_img["src"]

    # 评分
    score_el = soup2.find("span", class_="score")
    if score_el:
        value_el = score_el.find("span", class_="value")
        if value_el:
            try:
                meta["rating"] = float(value_el.get_text(strip=True))
            except ValueError:
                pass

    # 信息面板
    for panel in soup2.find_all("div", class_="panel-block"):
        text = panel.get_text()
        # 日期
        if "日期" in text:
            date_match = re.search(r"(\d{4}-\d{2}-\d{2})", text)
            if date_match:
                meta["release_date"] = date_match.group(1)
        # 時長
        elif "時長" in text:
            dur_match = re.search(r"(\d+)\s*分鐘", text)
            if dur_match:
                meta["duration"] = int(dur_match.group(1))
        # 片商
        elif "片商" in text:
            a = panel.find("a")
            if a:
                meta["studio"] = a.get_text(strip=True)
        # 演員
        elif "演員" in text:
            actresses = []
            for a in panel.find_all("a"):
                name = a.get_text(strip=True)
                if name:
                    actresses.append(name)
            if actresses:
                meta["actresses"] = actresses
        # 類別
        elif "類別" in text:
            genres = []
            for a in panel.find_all("a"):
                tag = a.get_text(strip=True)
                if tag:
                    genres.append(tag)
            if genres:
                meta["genres"] = genres

    return meta if meta.get("title") else None


# ==================== API 端点 ====================

@app.before_request
def check_api_key():
    """API Key 认证中间件"""
    if API_KEY and request.endpoint != "health":
        key = request.headers.get("X-API-Key", "")
        if key != API_KEY:
            return jsonify({"error": "Unauthorized"}), 401


@app.route("/health", methods=["GET"])
def health():
    """健康检查"""
    return jsonify({"status": "ok", "service": "nowen-adult-scraper"})


@app.route("/api/scrape", methods=["POST"])
def scrape():
    """
    番号刮削接口
    
    Request Body:
    {
        "code": "SSIS-001",
        "sources": ["javdb", "javbus"]  // 可选，指定数据源优先级
    }
    
    Response:
    {
        "code": "SSIS-001",
        "title": "...",
        "cover": "https://...",
        "actresses": ["..."],
        "studio": "...",
        "genres": ["..."],
        "release_date": "2024-01-01",
        "duration": 120,
        "rating": 4.5,
        "source": "javdb"
    }
    """
    data = request.get_json()
    if not data or not data.get("code"):
        return jsonify({"error": "缺少 code 参数"}), 400

    code = data["code"].upper().strip()
    sources = data.get("sources", ["javbus", "javdb"])

    logger.info(f"收到刮削请求: {code}, 数据源: {sources}")

    # 按优先级尝试各数据源
    scrapers = {
        "javbus": scrape_javbus,
        "javdb": scrape_javdb,
    }

    for source in sources:
        if source in scrapers:
            try:
                result = scrapers[source](code)
                if result:
                    logger.info(f"刮削成功: {code} -> {result.get('title', 'N/A')} (来源: {source})")
                    return jsonify(result)
            except Exception as e:
                logger.error(f"{source} 刮削异常: {e}")
            
            # 数据源间间隔
            time.sleep(random.uniform(1.5, 3))

    return jsonify({"error": f"所有数据源均未找到番号 {code} 的元数据"}), 404


@app.route("/api/search", methods=["GET"])
def search():
    """
    番号搜索接口（返回搜索结果列表）
    
    Query Params:
        q: 搜索关键词
        source: 数据源（javbus/javdb）
    """
    query = request.args.get("q", "").strip()
    source = request.args.get("source", "javdb")

    if not query:
        return jsonify({"error": "缺少 q 参数"}), 400

    # TODO: 实现搜索功能（返回多个候选结果）
    return jsonify({"error": "搜索功能开发中"}), 501


if __name__ == "__main__":
    # 监听地址：优先读 SCRAPER_HOST / SCRAPER_PORT，兼容旧版 PORT
    host = os.environ.get("SCRAPER_HOST", "0.0.0.0")
    port = int(os.environ.get("SCRAPER_PORT", os.environ.get("PORT", 5000)))
    debug = os.environ.get("DEBUG", "false").lower() == "true"
    logger.info(f"番号刮削微服务启动: http://{host}:{port}")
    # 关闭 Flask 的 reloader，避免被 Go 进程多启动一份子进程
    app.run(host=host, port=port, debug=debug, use_reloader=False)
