#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
检查 Reply 字段编码问题
"""

import requests
import json

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"
TEST_PASSWORD = "password123"

def get_token():
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "email": TEST_EMAIL,
        "password": TEST_PASSWORD
    })
    if resp.status_code == 200:
        return resp.json()['data']['token']
    return None

def test():
    token = get_token()
    if not token:
        print("❌ 无法获取 token")
        return
    
    # 发送请求
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "message": "What is solmover?",
            "use_rag": False,
            "model": "gpt-5.1"
        }
    )
    
    print(f"Status: {resp.status_code}")
    
    # 原始文本
    print(f"\n原始响应文本 (前500字符):")
    print(repr(resp.text[:500]))
    
    # JSON 解析
    data = resp.json()
    
    if 'data' in data:
        reply_raw = data['data'].get('Reply')
        print(f"\nReply 字段:")
        print(f"  - 类型: {type(reply_raw)}")
        print(f"  - 长度: {len(reply_raw) if reply_raw else 0}")
        print(f"  - 值 (前100字符): {repr(reply_raw[:100] if reply_raw else '')}")
        
        if reply_raw:
            print(f"\n✅ Chat 正常工作！回复长度：{len(reply_raw)} 字符")
            print(f"回复内容:\n{reply_raw}\n")
        else:
            print(f"\n❌ Reply 字段为空或无法提取")

if __name__ == '__main__':
    test()
