package main

import "time"

// 这个文件定义了应用程序中使用的数据模型（或称为结构体）。
// 将所有数据结构集中在一个地方有助于保持代码的组织性和可读性。

// Connections 结构体对应从 Clash API `/connections` 端点返回的 JSON 响应的顶层结构。
// `json:"..."` 标签用于告诉 `encoding/json` 包如何在 JSON 和 Go 结构体之间映射字段。
type Connections struct {
	DownloadTotal uint64       `json:"downloadTotal"` // 总下载流量
	UploadTotal   uint64       `json:"uploadTotal"`   // 总上传流量
	Connections   []Connection `json:"connections"`   // 当前连接的列表
	Memory        uint         `json:"memory"`        // 内存使用情况（Clash 相关）
}

// Connection 结构体对应每个网络连接的详细信息。
// 这是从 Clash API 获取的主要数据单元。
type Connection struct {
	ID          string    `json:"id"`          // 连接的唯一标识符
	Metadata    Metadata  `json:"metadata"`    // 连接的元数据
	Upload      uint64    `json:"upload"`      // 此连接的上传流量（字节）
	Download    uint64    `json:"download"`    // 此连接的下载流量（字节）
	Start       time.Time `json:"start"`       // 连接开始的时间
	Chains      []string  `json:"chains"`      // 连接经过的代理链
	Rule        string    `json:"rule"`        // 匹配到的规则
	RulePayload string    `json:"rulePayload"` // 规则的附加信息
}

// Metadata 结构体包含了关于网络连接的更详细的元数据。
type Metadata struct {
	Network           string `json:"network"`           // 网络类型 (e.g., "tcp")
	Type              string `json:"type"`              // 连接类型 (e.g., "HTTP")
	SourceIP          string `json:"sourceIP"`          // 源 IP 地址
	DestinationIP     string `json:"destinationIP"`     // 目标 IP 地址
	SourcePort        string `json:"sourcePort"`        // 源端口
	DestinationPort   string `json:"destinationPort"`   // 目标端口
	Host              string `json:"host"`              // 目标主机名
	DNSMode           string `json:"dnsMode"`           // DNS 解析模式
	ProcessPath       string `json:"processPath"`       // 发起连接的进程路径
	RemoteDestination string `json:"remoteDestination"` // 远程目标地址（通常在 host 为空时使用）
}

// ConnectionInfo 是一个精简版的 Connection 结构体，专门用于 API 响应。
// 当前端请求连接列表时，我们不需要返回所有原始字段，只返回前端需要展示的数据，
// 这样可以减少网络传输的数据量。
type ConnectionInfo struct {
	Host     string    `json:"host"`     // 目标主机名
	SourceIP string    `json:"sourceIP"` // 源 IP 地址
	Upload   uint64    `json:"upload"`   // 上传流量
	Download uint64    `json:"download"` // 下载流量
	Start    time.Time `json:"start"`    // 开始时间
	Chains   []string  `json:"chains"`   // 代理链
}
