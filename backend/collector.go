package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetClashConnections 函数负责从 Clash API 获取实时的连接信息。
// 它还会对获取到的数据进行一些初步的清洗和处理。
// 参数:
//
//	apiURL: Clash API 的 /connections 端点 URL。
//	token: 用于 API 认证的 Token（secret）。
//	hostSuffixWhitelist: 一个字符串切片，包含主机后缀名单。
//
// 返回值:
//
//	*Connections: 一个指向 Connections 结构体的指针，包含了所有连接信息。
//	error: 如果在请求或处理过程中发生错误，则返回一个错误。
func GetClashConnections(apiURL, token string, hostSuffixWhitelist []string) (*Connections, error) {
	// 创建一个 HTTP 客户端。
	client := &http.Client{}
	// 创建一个新的 GET 请求。
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 `Authorization` 请求头，用于 Clash API 的认证。
	req.Header.Add("Authorization", "Bearer "+token)

	// 发送 HTTP 请求。
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Clash API 失败: %w", err)
	}
	// 使用 defer 确保在函数退出时关闭响应体，防止资源泄露。
	defer resp.Body.Close()

	// 检查 HTTP 响应的状态码。如果不是 200 OK，则表示请求失败。
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Clash API 返回错误状态: %s", resp.Status)
	}

	// 读取响应体的内容。
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 将 JSON 格式的响应体解析（Unmarshal）到 Connections 结构体中。
	var connections Connections
	if err := json.Unmarshal(body, &connections); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	// --- 数据清洗逻辑 ---
	// 遍历所有连接，进行一些数据规范化处理。
	for i := range connections.Connections {
		// 使用指针直接修改切片中的元素，效率更高。
		conn := &connections.Connections[i]

		// 1. 填充空的 host 字段。
		// 有时 Clash API 返回的 `host` 字段为空，但 `remoteDestination` 字段有值，
		// 我们可以用后者来填充前者。
		if conn.Metadata.Host == "" {
			conn.Metadata.Host = conn.Metadata.RemoteDestination
		}

		// 2. 应用主机后缀白名单。
		// 这个逻辑用于将一些 CDN 或视频服务的复杂子域名归一化。
		// 例如，将 `v22.lscache6.googlevideo.com` 替换为 `googlevideo.com`。
		for _, suffix := range hostSuffixWhitelist {
			if strings.HasSuffix(conn.Metadata.Host, suffix) {
				conn.Metadata.Host = suffix
				break // 匹配到第一个后缀后即可停止，避免不必要的循环。
			}
		}
	}

	// 返回处理过的连接信息。
	return &connections, nil
}
