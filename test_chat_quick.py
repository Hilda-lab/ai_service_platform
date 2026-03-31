#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
测试 Chat 功能
"""

import requests
import json
import time

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"
TEST_PASSWORD = "password123"

def log(msg, level="INFO"):
    print(f"[{level}] {msg}")

def get_token():
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "email": TEST_EMAIL,
        "password": TEST_PASSWORD
    })
    if resp.status_code == 200:
        return resp.json()['data']['token']
    return None

def test_chat(token, use_rag=False):
    """测试 Chat API"""
    mode = "RAG" if use_rag else "基础"
    log(f"\n测试 Chat ({mode})...")
    
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "message": "What is solmover?",
            "use_rag": use_rag,
            "model": "gpt-5.1"
        }
    )
    
    log(f"  - 状态码: {resp.status_code}")
    
    if resp.status_code != 200:
        log(f"  - 错误: {resp.text}", "ERROR")
        return
    
    data = resp.json()
    reply = data.get('data', {}).get('reply', '')
    session_id = data.get('data', {}).get('session_id')
    
    log(f"  - 回复长: {len(reply)} 字符")
    log(f"  - Session ID: {session_id}")
    
    if reply:
        log(f"  ✓ 回复内容: {reply[:100]}...")
    else:
        log(f"  ✗ 回复为空", "WARN")

def test_chat_stream(token):
    """测试流式 Chat"""
    log(f"\n测试流式 Chat...")
    
    resp = requests.post(f"{BASE_URL}/chat/completions/stream",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "message": "What is solmover?",
            "use_rag": True,
            "model": "gpt-5.1"
        },
        stream=True
    )
    
    log(f"  - 状态码: {resp.status_code}")
    
    if resp.status_code != 200:
        log(f"  - 错误: {resp.text}", "ERROR")
        return
    
    chunks = []
    for line in resp.iter_lines():
        if line:
            try:
                chunk_data = json.loads(line.decode('utf-8'))
                chunks.append(chunk_data)
            except:
                pass
    
    log(f"  - 接收chunks数: {len(chunks)}")
    
    if chunks:
        first = chunks[0]
        log(f"  - 第一个chunk: {json.dumps(first)[:100]}...")
    else:
        log(f"  ✗ 无chunks", "WARN")

def main():
    log("="*70)
    log("Chat 功能测试")
    log("="*70)
    
    token = get_token()
    if not token:
        log("无法获取token", "ERROR")
        return
    
    time.sleep(1)
    test_chat(token, use_rag=False)
    
    time.sleep(1)
    test_chat(token, use_rag=True)
    
    time.sleep(1)
    test_chat_stream(token)
    
    log("\n" + "="*70)
    log("测试完成")
    log("="*70)

if __name__ == '__main__':
    main()
