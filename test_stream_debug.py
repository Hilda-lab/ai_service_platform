#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
修复的流式 Chat 测试
"""

import requests
import json
import time

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"  
TEST_PASSWORD = "password123"

def get_token():
    resp = requests.post(f"{BASE_URL}/auth/login", json={"email": TEST_EMAIL, "password": TEST_PASSWORD})
    return resp.json()['data']['token'] if resp.status_code == 200 else None

def test_stream():
    token = get_token()
    
    print("[INFO] 测试流式 Chat...")
    
    resp = requests.post(f"{BASE_URL}/chat/completions/stream",
        headers={"Authorization": f"Bearer {token}"},
        json={"message": "What is solmover?", "use_rag": False, "model": "gpt-5.1"},
        stream=True
    )
    
    print(f"[INFO] 状态码: {resp.status_code}")
    
    chunks = 0
    content = ""
    
    for line in resp.iter_lines(decode_unicode=True):
        if not line:
            continue
        
        print(f"  原始行: {repr(line[:100])}")
        
        if line.startswith("data: "):
            payload = line[6:]
            print(f"  Payload: {repr(payload[:100])}")
            try:
                data = json.loads(payload)
                print(f"  解析: type={data.get('type')}")
                
                if data.get('type') == 'chunk':
                    chunks += 1
                    chunk_content = data.get('content', '')
                    content += chunk_content
                    print(f"    Chunk {chunks}: {repr(chunk_content[:50])}")
                    
                elif data.get('type') == 'done':
                    print(f"  ✓ Stream 完成")
                    
            except json.JSONDecodeError as e:
                print(f"  JSON 错误: {e}")
    
    print(f"\n[RESULT]")
    print(f"  - Chunks: {chunks}")
    print(f"  - Content length: {len(content)}")
    print(f"  - Content: {content[:200]}")

if __name__ == '__main__':
    test_stream()
