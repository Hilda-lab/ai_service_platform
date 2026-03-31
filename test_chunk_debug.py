#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
诊断文档分块、向量化、存储过程
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

def upload_and_check(token):
    """上传文档并检查详细过程"""
    test_content = "Solmover是Solana生态中的资产转移解决方案。" * 10  # 重复使内容更长
    
    log("上传文档...")
    resp = requests.post(f"{BASE_URL}/rag/documents",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "title": "Solmover完整测试",
            "content": test_content
        }
    )
    
    log(f"  - 响应状态码: {resp.status_code}")
    
    if resp.status_code != 200:
        log(f"  - 响应体: {resp.text}", "ERROR")
        return None
    
    data = resp.json()
    
    if 'data' not in data:
        log(f"  - 无法找到data字段: {data}", "ERROR")
        return None
    
    doc_data = data['data']
    document = doc_data.get('document', {})
    chunks = doc_data.get('chunks', [])
    
    doc_id = document.get('id')
    doc_title = document.get('title')
    doc_content = document.get('content', '')
    doc_content_len = len(doc_content)
    
    log(f"✓ 文档上传成功:")
    log(f"  - 文档ID: {doc_id}")
    log(f"  - 标题: {doc_title}")
    log(f"  - 内容长: {doc_content_len} 字符")
    log(f"  - 分块数: {len(chunks)}")
    
    for i, chunk in enumerate(chunks):
        chunk_id = chunk.get('id')
        chunk_content = chunk.get('content', '')
        chunk_len = len(chunk_content)
        log(f"    [{i+1}] ChunkID={chunk_id}, 长={chunk_len}, 内容={'[空]' if not chunk_content else chunk_content[:50]+'...'}")
    
    return doc_id

def list_and_analyze(token):
    """列出所有文档并分析"""
    log("\n列出所有文档...")
    resp = requests.get(f"{BASE_URL}/rag/documents",
        headers={"Authorization": f"Bearer {token}"}
    )
    
    if resp.status_code != 200:
        log(f"  - 失败: {resp.status_code}", "ERROR")
        return
    
    docs = resp.json().get('data', [])
    log(f"✓ 共有 {len(docs)} 个文档:")
    
    for doc in docs:
        doc_id = doc.get('id')
        title = doc.get('title')
        chunk_count = doc.get('chunk_count', 0)
        content_len = len(doc.get('content', ''))
        log(f"  - [ID={doc_id}] {title} ({chunk_count}分块, {content_len}字符)")

def test_retrieve_detailed(token):
    """详细测试检索过程"""
    log("\n详细测试检索过程...")
    
    queries = [
        "solmover",
        "Solana资产转移",
        "what is solmover"
    ]
    
    for query in queries:
        log(f"\n  查询: '{query}'")
        resp = requests.post(f"{BASE_URL}/rag/retrieve",
            headers={"Authorization": f"Bearer {token}"},
            json={"query": query, "top_k": 5}
        )
        
        if resp.status_code != 200:
            log(f"    ✗ 检索失败: {resp.status_code}", "ERROR")
            log(f"    响应: {resp.text}", "ERROR")
            continue
        
        resp_json = resp.json()
        results = resp_json.get('data', [])
        metrics = resp_json.get('metrics', {})
        
        log(f"    - 结果数: {len(results)}")
        log(f"    - Corpus大小: {metrics.get('corpus_size', '?')}")
        log(f"    - 匹配数: {metrics.get('matched_count', '?')}")
        
        if len(results) == 0:
            log(f"    ⚠ 无结果", "WARN")
        
        for i, result in enumerate(results):
            score = result.get('similarity_score', 'N/A')
            content = result.get('content', '')[:60]
            doc_id = result.get('document_id')
            log(f"      [{i+1}] score={score}, doc_id={doc_id}, content={content}...")

def main():
    log("="*70)
    log("文档分块与向量化诊断")
    log("="*70)
    
    token = get_token()
    if not token:
        log("无法获取token", "ERROR")
        return
    
    log("\n[1/3] 上传并检查文档...")
    doc_id = upload_and_check(token)
    
    time.sleep(2)
    
    log("\n[2/3] 列出并分析所有文档...")
    list_and_analyze(token)
    
    time.sleep(1)
    
    log("\n[3/3] 详细测试检索...")
    test_retrieve_detailed(token)
    
    log("\n" + "="*70)
    log("诊断完成")
    log("="*70)

if __name__ == '__main__':
    main()
