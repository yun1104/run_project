import json
import os
import sys
import urllib.request
import re

# 强制 UTF-8，避免 Windows 默认 GBK 导致中文乱码
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
if hasattr(sys.stderr, "reconfigure"):
    sys.stderr.reconfigure(encoding="utf-8")


def _read_input():
    raw = sys.stdin.read()
    if not raw:
        return {}
    raw = raw.lstrip("\ufeff")
    return json.loads(raw)


def _extract_json_text(content: str) -> str:
    text = (content or "").strip()
    if text.startswith("```json"):
        text = text[len("```json"):].strip()
    if text.startswith("```"):
        text = text[len("```"):].strip()
    if text.endswith("```"):
        text = text[:-3].strip()
    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end > start:
        return text[start:end + 1]
    return text


def _model_cfg():
    api_key = os.getenv("MODELSCOPE_API_KEY", "").strip() or "ms-dd4cdb20-b7a7-4e39-95ea-ae1b5f412d4d"
    base_url = os.getenv("MODELSCOPE_BASE_URL", "").strip() or "https://api-inference.modelscope.cn/v1"
    model = os.getenv("MODELSCOPE_MODEL", "").strip() or "Qwen/Qwen3-30B-A3B-Instruct-2507"
    return api_key, base_url, model


def _call_modelscope(messages, temperature=0.2):
    api_key, base_url, model = _model_cfg()
    req_body = {
        "model": model,
        "messages": messages,
        "temperature": temperature,
        "response_format": {"type": "json_object"},
    }
    payload = json.dumps(req_body, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url=base_url.rstrip("/") + "/chat/completions",
        data=payload,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Authorization": "Bearer " + api_key,
        },
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        body = resp.read().decode("utf-8", errors="ignore")
    parsed = json.loads(body)
    choices = parsed.get("choices", [])
    if not choices:
        raise RuntimeError("empty choices")
    content = choices[0].get("message", {}).get("content", "")
    return json.loads(_extract_json_text(content))


def _classify_intent(user_text: str) -> bool:
    messages = [
        {"role": "system", "content": "你是意图分类器。只输出JSON。"},
        {
            "role": "user",
            "content": (
                "判断这句话是否是点餐/外卖推荐需求。"
                "只要包含预算、口味、送达时间、点餐、外卖、推荐吃什么等意图，就判定为true。"
                "纯问候（你好/在吗）或普通闲聊判定为false。"
                "输出JSON：{\"is_order_intent\":true/false,\"reason\":\"...\"}。"
                "不要输出JSON之外内容。"
                "\n示例1：'你好' -> false"
                "\n示例2：'预算40内，偏辣，30分钟内送达' -> true"
                "\n示例3：'晚饭吃什么推荐一下' -> true"
                "\n用户输入：" + user_text
            ),
        },
    ]
    out = _call_modelscope(messages, temperature=0.0)
    return bool(out.get("is_order_intent", False))


def _order_hint_score(user_text: str) -> int:
    text = (user_text or "").lower()
    hints = [
        "预算", "送达", "分钟", "外卖", "点餐", "推荐", "吃什么", "晚饭", "午饭",
        "早餐", "夜宵", "辣", "清淡", "轻食", "川菜", "快餐", "饿", "想吃", "吃点",
        "takeaway", "order", "restaurant", "meal", "lunch", "dinner",
        "food", "delivery", "spicy", "budget",
    ]
    score = 0
    for h in hints:
        if h in text:
            score += 1
    return score


def _contains_strong_order_hint(user_text: str) -> bool:
    text = (user_text or "").lower()
    strong_hints = [
        "外卖", "点餐", "吃什么", "想吃", "吃点", "午饭", "晚饭", "夜宵", "早餐", "饿",
        "takeaway", "order food", "restaurant", "lunch", "dinner", "meal"
    ]
    for h in strong_hints:
        if h in text:
            return True
    return False


def _sanitize_llm_merchants(raw_merchants: list, candidates: list):
    by_id = {}
    for c in candidates or []:
        try:
            cid = int(c.get("id"))
        except Exception:
            continue
        by_id[cid] = c

    out = []
    for item in raw_merchants or []:
        try:
            cid = int(item.get("id"))
        except Exception:
            continue
        base = by_id.get(cid)
        if not base:
            continue
        allowed = [str(x).strip() for x in (base.get("tags") or []) if str(x).strip()]
        raw_dishes = item.get("dishes") or []
        dishes = []
        for d in raw_dishes:
            sd = str(d).strip()
            if not sd:
                continue
            if allowed and sd in allowed:
                dishes.append(sd)
        if not dishes:
            dishes = allowed[:3]
        out.append({
            "id": cid,
            "reason": str(item.get("reason", "") or "").strip() or "匹配你的需求与偏好",
            "dishes": dishes[:4],
        })
        if len(out) >= 3:
            break
    return out


def _build_safe_reply(selected: list, candidates: list):
    by_id = {}
    for c in candidates or []:
        try:
            by_id[int(c.get("id"))] = c
        except Exception:
            continue
    if not selected:
        return "已结合你的需求与偏好筛选附近真实商家。"
    chunks = []
    for item in selected[:3]:
        c = by_id.get(int(item["id"]))
        if not c:
            continue
        name = str(c.get("name", "") or "").strip()
        dishes = item.get("dishes") or []
        if dishes:
            chunks.append(f"{name}（推荐：{'、'.join(dishes[:3])}）")
        else:
            chunks.append(f"{name}（菜品待到店确认）")
    if not chunks:
        return "已结合你的需求与偏好筛选附近真实商家。"
    return "为你推荐附近真实商家：" + "；".join(chunks)


def _tokenize(text: str):
    text = (text or "").lower()
    return set(re.findall(r"[\u4e00-\u9fa5a-z0-9]+", text))


def _join_pref_terms(pref: dict) -> str:
    if not isinstance(pref, dict):
        return ""
    parts = []
    for k in ["categories", "cuisine_likes", "tastes", "dish_keywords", "avoid_foods", "price_range"]:
        v = pref.get(k)
        if isinstance(v, list):
            parts.extend([str(x) for x in v if str(x).strip()])
        elif isinstance(v, str) and v.strip():
            parts.append(v.strip())
    return " ".join(parts)


def _build_rag_docs(candidates: list, nearby_foods: list):
    docs = []
    for c in candidates or []:
        cid = c.get("id")
        if cid is None:
            continue
        tags = [str(x).strip() for x in (c.get("tags") or []) if str(x).strip()]
        fields = [
            str(c.get("name", "") or ""),
            str(c.get("category", "") or ""),
            " ".join(tags),
            str(c.get("reason", "") or ""),
            str(c.get("distance", "") or ""),
            str(c.get("avg_price", "") or ""),
            str(c.get("rating", "") or ""),
        ]
        docs.append(
            {
                "id": int(cid),
                "content": " ".join([x for x in fields if x]),
                "merchant": c,
                "dishes": tags[:8],
            }
        )

    id_to_doc = {d["id"]: d for d in docs}
    for f in nearby_foods or []:
        try:
            rid = int(f.get("id"))
        except Exception:
            continue
        raw_dishes = f.get("dishes") or []
        dishes = [str(x).strip() for x in raw_dishes if str(x).strip()]
        if not dishes:
            continue
        target = id_to_doc.get(rid)
        if not target:
            continue
        merged = set(target.get("dishes") or [])
        merged.update(dishes)
        target["dishes"] = list(merged)[:12]
        target["content"] = (target["content"] + " " + " ".join(dishes)).strip()
    return docs


def _retrieve_context(user_text: str, pref: dict, docs: list, k: int = 8):
    query_text = (user_text or "").strip() + " " + _join_pref_terms(pref)
    q_tokens = _tokenize(query_text)
    if not q_tokens:
        return docs[:k]

    scored = []
    for d in docs:
        d_tokens = _tokenize(d.get("content", ""))
        if not d_tokens:
            score = 0.0
        else:
            overlap = len(q_tokens & d_tokens)
            score = overlap / (len(d_tokens) ** 0.5)
            if d.get("merchant", {}).get("category") and str(d["merchant"]["category"]).lower() in query_text.lower():
                score += 1.2
        scored.append((score, d))
    scored.sort(key=lambda x: x[0], reverse=True)
    top = [d for s, d in scored if s > 0][:k]
    if top:
        return top
    return [d for _, d in scored[:k]]


def _recommend(user_text: str, pref: dict, candidates: list, nearby_foods: list):
    pref_text = json.dumps(pref or {}, ensure_ascii=False)
    docs = _build_rag_docs(candidates, nearby_foods)
    retrieved = _retrieve_context(user_text, pref, docs, k=8)
    rag_context = []
    for d in retrieved:
        m = d.get("merchant", {})
        rag_context.append(
            {
                "id": d.get("id"),
                "name": m.get("name"),
                "category": m.get("category"),
                "rating": m.get("rating"),
                "avg_price": m.get("avg_price"),
                "distance": m.get("distance"),
                "reason": m.get("reason"),
                "dishes": d.get("dishes", [])[:8],
            }
        )
    rag_text = json.dumps(rag_context, ensure_ascii=False)
    messages = [
        {"role": "system", "content": "你是外卖推荐助手，需要同时推荐店铺和具体菜品。只输出JSON，禁止输出其他内容。"},
        {
            "role": "user",
            "content": (
                "根据用户需求和偏好，从检索上下文中推荐最多3家并为每家推荐2-4道菜。\n"
                "必须遵守：店铺id只能来自RAG上下文；禁止编造不存在的店铺。\n"
                "菜品必须来自RAG上下文里的 dishes；若没有可用菜品则返回空数组，不得臆造。\n"
                "输出格式（严格JSON，不要有多余文字）：\n"
                "{\"reply\":\"...(回复中需自然地提及具体菜名，不只说推荐哪家店)\","
                "\"merchants\":[{\"id\":10001,\"reason\":\"推荐理由\","
                "\"dishes\":[\"菜品A\",\"菜品B\",\"菜品C\"]}]}\n"
                "用户输入：" + user_text +
                "\n用户偏好：" + pref_text +
                "\nRAG检索上下文：" + rag_text
            ),
        },
    ]
    out = _call_modelscope(messages, temperature=0.2)
    selected = _sanitize_llm_merchants(out.get("merchants", []), candidates)
    return {
        "reply": _build_safe_reply(selected, candidates),
        "merchants": selected,
    }


def _normal_chat(user_text: str):
    messages = [
        {"role": "system", "content": "你是友好的中文助手，只输出JSON。"},
        {
            "role": "user",
            "content": (
                "请正常回复用户，不要做外卖推荐。"
                "输出JSON：{\"reply\":\"...\"}。不要输出JSON之外内容。"
                "\n用户输入：" + user_text
            ),
        },
    ]
    out = _call_modelscope(messages, temperature=0.4)
    reply = str(out.get("reply", "") or "").strip()
    if not reply:
        reply = "你好呀，我在。你可以告诉我预算、口味、送达时间，我再给你推荐外卖。"
    return {"reply": reply, "merchants": []}


def main():
    data = _read_input()
    user_text = str(data.get("requirement", "") or "").strip()
    pref = data.get("preference", {}) if data.get("has_pref") else {}
    if isinstance(pref, dict):
        if pref.get("cuisine_likes") is None:
            pref["cuisine_likes"] = []
        if pref.get("avoid_foods") is None:
            pref["avoid_foods"] = []
    candidates = data.get("candidates", [])
    nearby_foods = data.get("nearby_foods", [])

    try:
        hint_score = _order_hint_score(user_text)
        has_strong_hint = _contains_strong_order_hint(user_text)
        is_order = _classify_intent(user_text)
        # 非外卖语义直接走普通对话，避免误推荐
        if hint_score == 0:
            is_order = False
        # 明确点餐语义兜底，避免误判成闲聊
        elif has_strong_hint:
            is_order = True
        # 命中多个点餐特征时兜底走点餐
        elif hint_score >= 2:
            is_order = True
        if is_order:
            result = _recommend(user_text, pref, candidates, nearby_foods)
            out = {
                "is_order_intent": True,
                "reply": result.get("reply", ""),
                "merchants": result.get("merchants", []),
            }
        else:
            result = _normal_chat(user_text)
            out = {
                "is_order_intent": False,
                "reply": result.get("reply", ""),
                "merchants": [],
            }
        sys.stdout.write(json.dumps(out, ensure_ascii=False))
    except Exception as e:
        sys.stderr.write(str(e))
        sys.exit(1)


if __name__ == "__main__":
    main()
