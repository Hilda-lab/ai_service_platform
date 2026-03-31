#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
完整的 Chat + RAG 功能测试
"""

import requests
import json
import time

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"
TEST_PASSWORD = "password123"

def log(msg, level="INFO"):
    prefix = ""
    if level == "SUCCESS":
        prefix = "✓ "
    elif level == "FAIL":
        prefix = "✗ "
    elif level == "WARN":
        prefix = "⚠ "
    print(f"[{level}] {prefix}{msg}")

def get_token():
    resp = requests.post(f"{BASE_URL}/auth/login", json={"email": TEST_EMAIL, "password": TEST_PASSWORD})
    if resp.status_code == 200:
        return resp.json()['data']['token']
    return None

def upload_doc(token):
    """上传测试文档"""
    log("上传文档...", "INFO")
    
    test_content = "Solmover是Solana生态中关于资产转移的解决方案。" * 5
    
    resp = requests.post(f"{BASE_URL}/rag/documents",
        headers={"Authorization": f"Bearer {token}"},
        json={"title": "Solmover完整测试", "content": test_content}
    )
    
    if resp.status_code != 200:
        log(f"文档上传失败: {resp.status_code}", "FAIL")
        return None
    
    doc_id = resp.json()['data']['document']['id']
    log(f"文档上传成功 (ID={doc_id})", "SUCCESS")
    return doc_id

def test_chat(token, use_rag=False, wait_before=1):
    """测试 Chat"""
    time.sleep(wait_before)
    
    mode = "RAG" if use_rag else "基础"
    log(f"测试 Chat ({mode})...", "INFO")
    
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "message": "What is solmover?",
            "use_rag": use_rag,
            "model": "gpt-5.1"
        }
    )
    
    if resp.status_code != 200:
        log(f"Chat 失败: {resp.status_code}", "FAIL")
        return False
    
    reply = resp.json()['data'].get('reply', '')
    reply_len = len(reply)
    
    if reply_len > 0:
        log(f"Chat 成功 ({mode}), 回复长度: {reply_len} 字符", "SUCCESS")
        log(f"  回复摘要: {reply[:100]}...", "INFO")
        return True
    else:
        log(f"Chat 返回空回复 ({mode})", "FAIL")
        return False

def test_chat_stream(token, use_rag=False):
    """测试 Chat 流式"""
    log(f"测试 Chat 流式 (RAG={use_rag})...", "INFO")
    
    resp = requests.post(f"{BASE_URL}/chat/completions/stream",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "message": "What is solmover?",
            "use_rag": use_rag,
            "model": "gpt-5.1"
        },
        stream=True
    )
    
    if resp.status_code != 200:
        log(f"Stream 失败: {resp.status_code}", "FAIL")
        return False
    
    chunks = 0
    content = ""
    
    for line in resp.iter_lines():
        if line and line.startswith(b'data: '):
            try:
                chunk_data = json.loads(line[6:])
                if chunk_data.get('type') == 'chunk':
                    chunks += 1
                    content += chunk_data.get('content', '')
                elif chunk_data.get('type') == 'done':
                    log(f"Stream 完成 (chunksStream 完成 (chunks={chunks}), 内容长={len(content)}", "SUCCESS")
            except:
                pass
    
    if chunks > 0:
        log(f"✓ Stream 流式成功, 共 {chunks} chunks, 内容长={len(content)}", "SUCCESS")
        return True
    else:
        log(f"✗ Stream 无 chunks", "FAIL")
        return False

def main():
    log("="*60, "INFO")
    log("完整 Chat + RAG 功能测试", "INFO")
    log("="*60, "INFO")
    
    token = get_token()
    if not token:
        log("无法获取 token", "FAIL")
        return
    
    results = []
    
    # 1. 上传文档
    doc_id = upload_doc(token)
    time.sleep(2)
    
    # 2. 基础 Chat
    results.append(("Chat 基础", test_chat(token, use_rag=False)))
    
    # 3. RAG Chat
    results.append(("Chat + RAG", test_chat(token, use_rag=True)))
    
    # 4. 流式 Chat
    results.append(("Chat 流式 (无RAG)", test_chat_stream(token, use_rag=False)))
    
    # 5. 流式 Chat + RAG
    results.append(("Chat 流式 (RAG)", test_chat_stream(token, use_rag=True)))
    
    # 测试摘要
    log("="*60, "INFO")
    log("测试摘要:", "INFO")
    for name, result in results:
        status = "✓ 通过" if result else "✗ 失败"
        print(f"  {status} {name}")
    
    passed = sum(1 for _, r in results if r)
    log(f"\n总计: {passed}/{len(results)} 通过", "INFO")

if __name__ == '__main__':
    main()
