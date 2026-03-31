#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
详细诊断 Chat 空回复问题
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

def test_chat_detailed(token):
    """详细测试 Chat 响应"""
    log("\n发送 Chat 请求...")
    
    payload = {
        "message": "What is solmover?",
        "use_rag": False,
        "model": "gpt-5.1"
    }
    
    log(f"  请求体: {json.dumps(payload)}")
    
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json=payload
    )
    
    log(f"\n响应状态码: {resp.status_code}")
    log(f"响应头: {dict(resp.headers)}")
    
    try:
        resp_json = resp.json()
        log(f"\n完整响应体:")
        print(json.dumps(resp_json, indent=2, ensure_ascii=False))
        
        # 详细分析
        if 'data' in resp_json:
            data = resp_json['data']
            log(f"\n数据字段: {json.dumps(data, indent=2, ensure_ascii=False)}")
            
            reply = data.get('reply', '')
            log(f"\nReply 字段值: '{reply}' (长度={len(reply)})")
            
            if not reply:
                log("⚠️ 回复为空!", "WARN")
                
                # 检查其他字段
                if 'error' in data:
                    log(f"  错误: {data['error']}", "ERROR")
                if 'message' in data:
                    log(f"  消息: {data['message']}")
    except Exception as e:
        log(f"错误: {e}", "ERROR")
        log(f"响应文本: {resp.text}")

def main():
    log("="*70)
    log("Chat 空回复详细诊断")
    log("="*70)
    
    token = get_token()
    if not token:
        log("无法获取token", "ERROR")
        return
    
    time.sleep(1)
    test_chat_detailed(token)
    
    log("\n" + "="*70)

if __name__ == '__main__':
    main()
