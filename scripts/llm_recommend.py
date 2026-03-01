import json
import os
import sys
import urllib.request


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


def _recommend(user_text: str, pref: dict, candidates: list):
    pref_text = json.dumps(pref or {}, ensure_ascii=False)
    cand_text = json.dumps(candidates or [], ensure_ascii=False)
    messages = [
        {"role": "system", "content": "你是外卖推荐助手。只输出JSON。"},
        {
            "role": "user",
            "content": (
                "请基于用户需求、用户偏好和候选商家，推荐最多3个商家并排序。"
                "输出JSON：{\"reply\":\"...\",\"merchants\":[{\"id\":10001,\"reason\":\"...\"}]}。"
                "商家id只能从候选商家中选择。不要输出JSON之外内容。"
                "\n用户输入：" + user_text +
                "\n用户偏好：" + pref_text +
                "\n候选商家：" + cand_text
            ),
        },
    ]
    out = _call_modelscope(messages, temperature=0.2)
    return {
        "reply": str(out.get("reply", "") or "").strip() or "已根据你的需求给出推荐。",
        "merchants": out.get("merchants", []),
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
    candidates = data.get("candidates", [])

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
            result = _recommend(user_text, pref, candidates)
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
