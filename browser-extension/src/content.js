(() => {
  const VIDEO_CODE_RE = /\b([A-Z]{2,10})-?(\d{3,8})\b/i;

  const clean = (value) =>
    (value || "")
      .replace(/\s+/g, " ")
      .replace(/\u00a0/g, " ")
      .trim();

  const absUrl = (value) => {
    const raw = clean(value);
    if (!raw) return "";
    if (raw.startsWith("//")) return `${location.protocol}${raw}`;
    try {
      return new URL(raw, location.href).href;
    } catch {
      return raw;
    }
  };

  const uniq = (items) => {
    const seen = new Set();
    return items
      .map(clean)
      .filter((item) => {
        if (!item || seen.has(item)) return false;
        seen.add(item);
        return true;
      });
  };

  const codeFromText = (text) => {
    const match = clean(text).toUpperCase().match(VIDEO_CODE_RE);
    return match ? `${match[1].toUpperCase()}-${match[2]}` : "";
  };

  const numberFromText = (text) => {
    const match = clean(text).match(/(\d+(?:\.\d+)?)/);
    return match ? Number(match[1]) : 0;
  };

  const hashText = asyncTextHashSeed => {
    let hash = 0;
    for (let i = 0; i < asyncTextHashSeed.length; i += 1) {
      hash = ((hash << 5) - hash + asyncTextHashSeed.charCodeAt(i)) | 0;
    }
    return `local-${Math.abs(hash)}`;
  };

  const textWithoutLabels = (node) => {
    const clone = node.cloneNode(true);
    clone.querySelectorAll("strong, span.header, .header").forEach((el) => el.remove());
    return clean(clone.textContent);
  };

  const parseJavBus = () => {
    const titleText = clean(document.querySelector("h3")?.textContent);
    const code = codeFromText(titleText) || codeFromText(location.pathname);
    const actorPhotos = {};
    const actresses = [];

    document.querySelectorAll("a.avatar-box").forEach((el) => {
      const name = clean(el.querySelector("span")?.textContent || el.textContent);
      const photo = absUrl(el.querySelector("img")?.getAttribute("src"));
      if (name) {
        actresses.push(name);
        if (photo && !/nowprinting|default/i.test(photo)) actorPhotos[name] = photo;
      }
    });

    const meta = {
      code,
      title: clean(titleText.replace(code, "")),
      original_title: titleText,
      cover: absUrl(document.querySelector(".bigImage")?.getAttribute("href")) ||
        absUrl(document.querySelector(".bigImage img")?.getAttribute("src")),
      thumb: "",
      actresses: uniq(actresses),
      actor_photos: actorPhotos,
      studio: "",
      label: "",
      series: "",
      genres: uniq([...document.querySelectorAll("span.genre a, a[href*='/genre/']")].map((el) => el.textContent)),
      release_date: "",
      duration: 0,
      rating: 0,
      trailer: absUrl(document.querySelector("video[src]")?.getAttribute("src")),
      extra_fanart: uniq([...document.querySelectorAll("a.sample-box")].map((el) => absUrl(el.getAttribute("href")))),
      short_reviews: [],
      director: "",
      plot: "",
      source: "javbus",
    };

    document.querySelectorAll(".col-md-3.info p, .info p, p").forEach((p) => {
      const text = clean(p.textContent);
      const value = textWithoutLabels(p).replace(/^[:：]/, "").trim();
      if (/發行日期|发行日期|発売日/.test(text)) meta.release_date = value.match(/\d{4}-\d{2}-\d{2}/)?.[0] || value;
      if (/長度|长度|収録時間/.test(text)) meta.duration = numberFromText(value);
      if (/導演|导演|監督/.test(text)) meta.director = clean(p.querySelector("a")?.textContent || value);
      if (/製作商|制作商|メーカー/.test(text)) meta.studio = clean(p.querySelector("a")?.textContent || value);
      if (/發行商|发行商|レーベル/.test(text)) meta.label = clean(p.querySelector("a")?.textContent || value);
      if (/系列|シリーズ/.test(text)) meta.series = clean(p.querySelector("a")?.textContent || value);
    });

    return meta;
  };

  const parseJavDBShortReviews = () => {
    const selectors = [
      ".review-item",
      ".reviews .item",
      ".review-list .item",
      ".movie-reviews .item",
      ".comments .comment",
      ".comment-item",
    ].join(", ");
    const blocks = [...document.querySelectorAll(selectors)];
    const seen = new Set();
    return blocks
      .map((block) => {
        const contentEl = block.querySelector(".content, .review-content, .message-body, .message, p") || block;
        const authorEl = block.querySelector(".username, .user-name, .author, .name, a[href*='/users/']");
        const linkEl = block.querySelector("a[href*='/reviews/'], a[href*='#review'], a[href*='/posts/']");
        const content = clean(contentEl.textContent);
        const author = clean(authorEl?.textContent || "");
        const rating = numberFromText(block.querySelector(".score .value, .rating, .stars")?.textContent || "");
        const likesText = clean(block.querySelector(".vote, .likes, .like-count")?.textContent || block.textContent);
        const likesMatch = likesText.match(/(?:有用|赞|like|likes)?\s*(\d+)/i);
        const sourceId = clean(block.getAttribute("data-id") || linkEl?.getAttribute("href") || hashText(`${author}\n${content}`));
        return {
          id: sourceId,
          author,
          content,
          rating,
          likes: likesMatch ? Number(likesMatch[1]) : 0,
        };
      })
      .filter((review) => {
        if (!review.content || review.content.length < 2 || seen.has(review.id)) return false;
        seen.add(review.id);
        return true;
      })
      .slice(0, 80);
  };

  const javdbPanelValue = (labels) => {
    for (const block of document.querySelectorAll(".panel-block, .movie-panel-info .item, .video-meta-panel .item")) {
      const text = clean(block.textContent);
      if (!labels.some((label) => text.includes(label))) continue;
      return {
        text,
        value: textWithoutLabels(block).replace(/^[:：]/, "").trim(),
        links: [...block.querySelectorAll("a")].map((el) => clean(el.textContent)).filter(Boolean),
      };
    }
    return { text: "", value: "", links: [] };
  };

  const parseJavDB = () => {
    const titleText = clean(document.querySelector("h2.title strong, h2.title, .video-detail h2")?.textContent);
    const numberBlock = javdbPanelValue(["番號", "番号", "ID"]);
    const dateBlock = javdbPanelValue(["日期", "発売日"]);
    const durationBlock = javdbPanelValue(["時長", "时长", "収録時間"]);
    const directorBlock = javdbPanelValue(["導演", "导演", "監督"]);
    const studioBlock = javdbPanelValue(["片商", "メーカー"]);
    const labelBlock = javdbPanelValue(["發行", "发行", "レーベル"]);
    const seriesBlock = javdbPanelValue(["系列", "シリーズ"]);
    const actorBlock = javdbPanelValue(["演員", "演员", "出演者"]);
    const genreBlock = javdbPanelValue(["類別", "类别", "ジャンル"]);
    const actorPhotos = {};

    document.querySelectorAll("a[href*='/actors/'], a[href*='/actors/']").forEach((el) => {
      const name = clean(el.textContent);
      const img = el.querySelector("img") || el.closest(".actor-section, .panel-block, .actor-box")?.querySelector("img");
      const photo = absUrl(img?.getAttribute("src") || img?.getAttribute("data-src"));
      if (name && photo) actorPhotos[name] = photo;
    });

    const cover = document.querySelector("img.video-cover, .cover img, .column-video-cover img");
    const rating = clean(document.querySelector(".score .value, .score")?.textContent);
    const code = codeFromText(numberBlock.value) || codeFromText(titleText) || codeFromText(location.pathname);

    return {
      code,
      title: clean(titleText.replace(code, "")),
      original_title: titleText,
      cover: absUrl(cover?.getAttribute("src") || cover?.getAttribute("data-src")),
      thumb: "",
      actresses: uniq(actorBlock.links),
      actor_photos: actorPhotos,
      studio: studioBlock.links[0] || studioBlock.value,
      label: labelBlock.links[0] || labelBlock.value,
      series: seriesBlock.links[0] || seriesBlock.value,
      genres: uniq(genreBlock.links),
      release_date: dateBlock.value.match(/\d{4}-\d{2}-\d{2}/)?.[0] || dateBlock.value,
      duration: numberFromText(durationBlock.value),
      rating: numberFromText(rating),
      trailer: absUrl(document.querySelector("video[src]")?.getAttribute("src")),
      extra_fanart: uniq(
        [...document.querySelectorAll("a.tile-item, .preview-images a, .sample-waterfall a")]
          .map((el) => absUrl(el.getAttribute("href")))
          .filter((url) => /\.(jpe?g|png|webp)(?:$|\?)/i.test(url)),
      ),
      short_reviews: parseJavDBShortReviews(),
      director: directorBlock.links[0] || directorBlock.value,
      plot: clean(document.querySelector(".video-description, .review-panel, .message-body")?.textContent),
      source: "javdb",
    };
  };

  const extractMetadata = () => {
    const host = location.hostname.toLowerCase();
    if (host.includes("javbus")) return parseJavBus();
    if (host.includes("javdb")) return parseJavDB();
    throw new Error("当前页面不是 JavDB 或 JavBus");
  };

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== "NOWEN_EXTRACT_ADULT_METADATA") return false;
    try {
      const metadata = extractMetadata();
      if (!metadata.code) throw new Error("未能识别番号");
      sendResponse({ ok: true, metadata });
    } catch (error) {
      sendResponse({ ok: false, error: error.message || String(error) });
    }
    return true;
  });
})();
