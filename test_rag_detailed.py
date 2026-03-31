#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
深度RAG诊断脚本
比较有RAG和无RAG的响应，找出问题所在
"""

import requests
import json
import time

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"
TEST_PASSWORD = "password123"

def log(msg, level="INFO"):
    timestamp = time.strftime("%H:%M:%S")
    print(f"[{timestamp}][{level}] {msg}")

def get_token():
    """获取认证token"""
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "email": TEST_EMAIL,
        "password": TEST_PASSWORD
    })
    if resp.status_code == 200:
        return resp.json()['data']['token']
    return None

def upload_test_doc(token):
    """上传测试文档"""
    doc_content = """Solmover是区块链资产转移领域的重要术语。
    Solmover结合了Solana生态(Sol)和资产转移(Mover)的概念。
    Solmover应用场景包括：跨链转账、NFT转移、流动性挖矿。
    Solmover技术涉及智能合约、密钥管理和交易验证。"""
    
    resp = requests.post(f"{BASE_URL}/rag/documents",
        headers={"Authorization": f"Bearer {token}"},
        json={"title": "Solmover详解", "content": doc_content}
    )
    
    if resp.status_code == 200:
        return resp.json()['data']['document']['id']
    return None

def test_rag_retrieval(token, query="什么是solmover"):
    """测试RAG检索"""
    log(f"测试RAG检索: {query}")
    
    resp = requests.post(f"{BASE_URL}/rag/retrieve",
        headers={"Authorization": f"Bearer {token}"},
        json={"query": query, "top_k": 3}
    )
    
    if resp.status_code == 200:
        results = resp.json().get('data', [])
        log(f"  ✓ 检索成功，得到 {len(results)} 个结果")
        for i, result in enumerate(results):
            score = result.get('similarity_score', 'N/A')
            content = result.get('content', '')[:80]
            log(f"  [{i+1}] 分数={score}, 内容={content}...")
        return results
    else:
        log(f"  ✗ 检索失败: {resp.status_code}")
        return []

def test_chat_rag_mode(token, query, use_rag=True):
    """测试Chat (有/无RAG模式)"""
    mode_str = "WITH RAG" if use_rag else "WITHOUT RAG"
    log(f"\n测试Chat {mode_str}: {query}")
    
    payload = {
        "provider": "openai",
        "model": "gpt-5.1",
        "message": query,
        "use_rag": use_rag
    }
    
    log(f"  - 请求体: use_rag={use_rag}")
    
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json=payload,
        timeout=30
    )
    
    log(f"  - 响应状态: {resp.status_code}")
    
    if resp.status_code == 200:
        data = resp.json().get('data', {})
        reply = data.get('reply', '')
        session_id = data.get('session_id')
        
        log(f"  ✓ Chat成功")
        log(f"  - Session ID: {session_id}")
        log(f"  - 回复长度: {len(reply)} 字符")
        
        if len(reply) == 0:
            log(f"  ⚠ 警告: 回复为空!", "WARN")
        else:
            # 检查是否包含关键词
            keywords = ['solmover', 'sol', 'mover', 'blockchain', 'chain', 'solana']
            found_keywords = [kw for kw in keywords if kw.lower() in reply.lower()]
            if found_keywords:
                log(f"  ✓ 包含关键词: {found_keywords}")
            else:
                log(f"  ⚠ 不包含期望的关键词", "WARN")
            
            # 显示回复内容（限制长度）
            preview = reply[:300] if len(reply) > 300 else reply
            log(f"  回复内容: {preview}...")
        
        return {
            'status': 'success',
            'reply': reply,
            'session_id': session_id,
            'has_content': len(reply) > 0
        }
    else:
        error_msg = resp.json().get('message', '未知错误')
        log(f"  ✗ Chat失败: {error_msg}")
        return {'status': 'error', 'message': error_msg}

def test_chat_stream(token, query, use_rag=True):
    """测试流式Chat模式"""
    mode_str = "WITH RAG" if use_rag else "WITHOUT RAG"
    log(f"\n测试流式Chat {mode_str}: {query}")
    
    payload = {
        "provider": "openai",
        "model": "gpt-5.1",
        "message": query,
        "use_rag": use_rag
    }
    
    resp = requests.post(f"{BASE_URL}/chat/completions/stream",
        headers={"Authorization": f"Bearer {token}"},
        json=payload,
        stream=True,
        timeout=30
    )
    
    log(f"  - 流式响应状态: {resp.status_code}")
    
    if resp.status_code == 200:
        content_parts = []
        chunk_count = 0
        session_id = None
        
        if resp.raw:
            resp.raw.read = lambda size: resp.raw.read(size)
        
        for line in resp.iter_lines():
            if not line:
                continue
            
            line_str = line.decode('utf-8') if isinstance(line, bytes) else line
            if line_str.startswith('data:'):
                try:
                    json_str = line_str[5:].strip()
                    event = json.loads(json_str)
                    
                    if event.get('type') == 'chunk' and event.get('content'):
                        content_parts.append(event['content'])
                        chunk_count += 1
                    elif event.get('type') == 'done':
                        session_id = event.get('session_id')
                except:
                    pass
        
        full_content = ''.join(content_parts)
        log(f"  ✓ 流式成功")
        log(f"  - 收到 {chunk_count} 个chunks")
        log(f"  - 总内容长: {len(full_content)} 字符")
        log(f"  - Session ID: {session_id}")
        
        if len(full_content) > 0:
            preview = full_content[:300]
            log(f"  内容预览: {preview}...")
        
        return {
            'status': 'success',
            'content': full_content,
            'chunks': chunk_count,
            'session_id': session_id
        }
    else:
        log(f"  ✗ 流式失败: {resp.status_code}")
        return {'status': 'error', 'code': resp.status_code}

def main():
    log("="*70)
    log("RAG深度诊断测试开始")
    log("="*70)
    
    # 1. 认证
    log("\n[1/7] 获取认证token...")
    token = get_token()
    if not token:
        log("无法获得token，测试中止", "ERROR")
        return
    log("✓ 获得token")
    
    time.sleep(1)
    
    # 2. 上传文档
    log("\n[2/7] 上传Solmover文档...")
    doc_id = upload_test_doc(token)
    if not doc_id:
        log("上传文档失败", "ERROR")
        return
    log(f"✓ 文档已上传 (ID={doc_id})")
    
    time.sleep(2)
    
    # 3. 测试RAG检索
    log("\n[3/7] 测试RAG检索...")
    rag_results = test_rag_retrieval(token)
    
    time.sleep(1)
    
    # 4. 无RAG的Chat
    log("\n[4/7] Chat无RAG模式...")
    result_no_rag = test_chat_rag_mode(token, "什么是solmover", use_rag=False)
    
    time.sleep(2)
    
    # 5. 有RAG的Chat (非流式)
    log("\n[5/7] Chat有RAG模式 (非流式)...")
    result_with_rag = test_chat_rag_mode(token, "什么是solmover", use_rag=True)
    
    time.sleep(2)
    
    # 6. 有RAG的流式Chat
    log("\n[6/7] Chat有RAG模式 (流式)...")
    result_stream = test_chat_stream(token, "什么是solmover", use_rag=True)
    
    # 7. 总结
    log("\n" + "="*70)
    log("诊断总结")
    log("="*70)
    
    log("\n📊 检索测试:")
    log(f"  - RAG能找到文档: {'✓' if len(rag_results) > 0 else '✗'}")
    
    log("\n💬 Chat非流式测试:")
    log(f"  - 无RAG回复: {'✓' if result_no_rag.get('has_content') else '✗'} ({len(result_no_rag.get('reply', ''))} 字符)")
    log(f"  - 有RAG回复: {'✓' if result_with_rag.get('has_content') else '✗'} ({len(result_with_rag.get('reply', ''))} 字符)")
    
    if result_no_rag.get('has_content') and result_with_rag.get('has_content'):
        no_rag = result_no_rag['reply'][:200]
        with_rag = result_with_rag['reply'][:200]
        if no_rag == with_rag:
            log(f"  ⚠ 两者回复内容相同，RAG可能未被应用!")
        else:
            log(f"  ✓ 两者回复内容不同，RAG已应用")
    
    log("\n📡 Chat流式测试:")
    if result_stream['status'] == 'success':
        log(f"  - 流式工作: ✓")
        log(f"  - 收到chunks: {result_stream['chunks']}")
    else:
        log(f"  - 流式工作: ✗")
    
    # 关键问题诊断
    log("\n🔍 问题诊断:")
    
    if len(rag_results) > 0:
        log("✓ RAG检索正常工作")
    else:
        log("✗ RAG检索异常")
    
    if not result_no_rag.get('has_content'):
        log("✗ Chat非流式基本不工作")
    elif not result_with_rag.get('has_content'):
        log("✗ Chat有RAG模式回复为空")
    else:
        log("✓ Chat非流式工作")
    
    if result_stream['status'] == 'error':
        log("✗ Chat流式工作异常")
    else:
        log("✓ Chat流式工作")

if __name__ == '__main__':
    main()
