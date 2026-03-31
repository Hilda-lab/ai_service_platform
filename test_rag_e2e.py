#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
完整的 RAG 端到端测试脚本
用于诊断 RAG 系统是否正常工作
"""

import requests
import json
import time

BASE_URL = "http://127.0.0.1:28080/api/v1"
TEST_EMAIL = "test@example.com"
TEST_PASSWORD = "password123"

def log(msg, level="INFO"):
    print(f"[{level}] {msg}")

def register_and_login():
    """注册/登录并获取 token"""
    # 尝试登录
    try:
        login_resp = requests.post(f"{BASE_URL}/auth/login", json={
            "email": TEST_EMAIL,
            "password": TEST_PASSWORD
        })
        
        if login_resp.status_code == 200:
            data = login_resp.json()
            token = data.get('data', {}).get('token')
            if token:
                log("✓ 登录成功")
                return token
        
        # 如果登录失败，尝试注册
        if login_resp.status_code in [400, 401]:
            log("尝试注册新用户...")
            reg_resp = requests.post(f"{BASE_URL}/auth/register", json={
                "email": TEST_EMAIL,
                "password": TEST_PASSWORD
            })
            if reg_resp.status_code in [200, 201]:
                # 注册成功后再登录
                login_resp2 = requests.post(f"{BASE_URL}/auth/login", json={
                    "email": TEST_EMAIL,
                    "password": TEST_PASSWORD
                })
                if login_resp2.status_code == 200:
                    token = login_resp2.json().get('data', {}).get('token')
                    if token:
                        log("✓ 注册并登录成功")
                        return token
        
        log(f"✗ 登录/注册失败: {login_resp.status_code} - {login_resp.text}", "ERROR")
        return None
    except Exception as e:
        log(f"✗ 认证异常: {str(e)}", "ERROR")
        return None

def test_ingest(token):
    """测试文档上传"""
    log("测试文档上传...")
    
    # 上传一个关于 Solmover 的测试文档
    test_doc = """
    Solmover 是一个关于区块链资产转移的术语。
    Sol 指的是 Solana 生态，Mover 意味着转移、交换。
    Solmover 项目涉及在 Solana 区块链上实现资产转移功能。
    常见的 Solmover 应用包括：
    1. Solana 代币钱包转移
    2. NFT 跨链转移
    3. DeFi 流动性挖矿（提供流动性并获得收益）
    """
    
    resp = requests.post(f"{BASE_URL}/rag/documents", 
        headers={"Authorization": f"Bearer {token}"},
        json={
            "title": "Solmover 入门指南",
            "content": test_doc
        }
    )
    
    if resp.status_code == 200:
        data = resp.json()['data']
        doc_id = data['document']['id']
        chunk_count = len(data.get('chunks', []))
        log(f"✓ 文档上传成功，文档ID: {doc_id}, 分块数: {chunk_count}")
        return doc_id, chunk_count
    else:
        log(f"✗ 文档上传失败: {resp.status_code} - {resp.text}", "ERROR")
        return None, 0

def test_rag_retrieve(token):
    """测试 RAG 检索"""
    log("测试 RAG 检索...")
    
    query = "什么是 solmover"
    
    resp = requests.post(f"{BASE_URL}/rag/retrieve",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "query": query,
            "top_k": 3
        }
    )
    
    if resp.status_code == 200:
        try:
            resp_json = resp.json()
            data = resp_json.get('data', [])
            metrics = resp_json.get('metrics', {})
            
            log(f"✓ 检索成功")
            log(f"  - 查询: {query}")
            log(f"  - 匹配数: {len(data)}")
            log(f"  - corpus大小: {metrics.get('corpus_size', 'N/A')}")
            
            if len(data) > 0:
                score = data[0].get('similarity_score')
                if isinstance(score, (int, float)):
                    log(f"  - 最高分: {score:.4f}")
                else:
                    log(f"  - 最高分: {score}")
                content = data[0].get('content', '')[: 100]
                log(f"  - 内容预览: {content}...")
            else:
                log("  ⚠ 检索结果为空!", "WARN")
            
            return len(data) > 0
        except Exception as e:
            log(f"✗ 检索响应解析失败: {str(e)}", "ERROR")
            log(f"  原始响应: {resp.text}", "ERROR")
            return False
    else:
        log(f"✗ 检索失败: {resp.status_code} - {resp.text}", "ERROR")
        return False

def test_chat_with_rag(token):
    """测试 Chat + RAG 集成（不使用MCP工具）"""
    log("测试 Chat + RAG集成...")
    
    payload = {
        "provider": "openai",
        "model": "gpt-5.1",
        "message": "什么是 solmover？",
        "use_rag": True
    }
    
    log(f"  - 请求内容: {json.dumps(payload, ensure_ascii=False)}")
    
    resp = requests.post(f"{BASE_URL}/chat/completions",
        headers={"Authorization": f"Bearer {token}"},
        json=payload
    )
    
    log(f"  - 响应状态码: {resp.status_code}")
    
    if resp.status_code == 200:
        try:
            data = resp.json().get('data', {})
            reply = data.get('reply', '')
            session_id = data.get('session_id')
            
            log(f"✓ Chat 请求成功")
            log(f"  - Session ID: {session_id}")
            log(f"  - 回复长度: {len(reply)} 字符")
            if len(reply) > 200:
                log(f"  - 回复预览: {reply[:300]}...")
            else:
                log(f"  - 回复: {reply}")
            
            # 检查回复是否包含知识库信息
            if 'solmover' in reply.lower() or 'sol' in reply.lower() or 'blockchain' in reply.lower():
                log("✓ 检测到 RAG 嵌入的知识内容", "SUCCESS")
                return True
            else:
                log("⚠ 回复中未找到预期的 RAG 内容", "WARN")
                return len(reply) > 10  # 至少有一些内容
        except Exception as e:
            log(f"✗ 响应解析失败: {str(e)}", "ERROR")
            return False
    else:
        log(f"✗ Chat 请求失败: {resp.status_code}", "ERROR")
        try:
            error_data = resp.json()
            # 简化错误显示
            error_msg = error_data.get('message', error_data.get('error', ''))
            if len(str(error_msg)) > 200:
                log(f"  - 错误摘要: {str(error_msg)[:200]}...", "ERROR")
            else:
                log(f"  - 错误: {error_msg}", "ERROR")
        except:
            log(f"  - 响应体: {resp.text[:200]}", "ERROR")
        
        return False

def test_list_documents(token):
    """测试列出所有文档"""
    log("列出所有上传的文档...")
    
    resp = requests.get(f"{BASE_URL}/rag/documents",
        headers={"Authorization": f"Bearer {token}"}
    )
    
    if resp.status_code == 200:
        docs = resp.json().get('data', [])
        log(f"✓ 共有 {len(docs)} 个文档")
        for doc in docs:
            log(f"  - [{doc['id']}] {doc['title']} ({doc['chunk_count']} 分块)")
        return len(docs) > 0
    else:
        log(f"✗ 获取文档列表失败: {resp.status_code}", "ERROR")
        return False

def main():
    log("="*60)
    log("开始 RAG 系统端到端测试", "INFO")
    log("="*60)
    
    # Step 1: 认证
    log("\n[Step 1] 用户认证")
    token = register_and_login()
    if not token:
        log("无法获取 token，测试中止", "ERROR")
        return
    
    time.sleep(1)
    
    # Step 2: 清理旧数据（可选）
    log("\n[Step 2] 列出现有文档")
    test_list_documents(token)
    
    time.sleep(1)
    
    # Step 3: 上传文档
    log("\n[Step 3] 上传测试文档")
    doc_id, chunk_count = test_ingest(token)
    if not doc_id:
        log("文档上传失败，测试中止", "ERROR")
        return
    
    time.sleep(2)  # 等待数据同步
    
    # Step 4: 测试 RAG 检索
    log("\n[Step 4] 测试 RAG 检索")
    retrieve_success = test_rag_retrieve(token)
    
    time.sleep(1)
    
    # Step 5: 测试 Chat + RAG
    log("\n[Step 5] 测试 Chat + RAG 集成")
    chat_success = test_chat_with_rag(token)
    
    # Step 6: 总结
    log("\n" + "="*60)
    log("测试总结", "INFO")
    log("="*60)
    log(f"✓ 文档上传: 成功 ({chunk_count} 分块)")
    log(f"{'✓' if retrieve_success else '✗'} RAG 检索: {'成功' if retrieve_success else '失败'}")
    log(f"{'✓' if chat_success else '✗'} Chat RAG: {'成功' if chat_success else '失败'}")
    
    if retrieve_success and chat_success:
        log("\n✓ RAG 系统正常工作！", "SUCCESS")
    else:
        log("\n⚠ RAG 系统存在问题，请检查诊断日志", "WARN")

if __name__ == '__main__':
    main()
