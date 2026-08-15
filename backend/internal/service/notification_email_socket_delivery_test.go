// 文件用途：REQ-23 真实 SMTP 投递的端到端运行时证据。
//
// 与其它邮件用例不同，这里不替换 sendMailWithDialer 缝，而是启动一个进程内的
// 最小 SMTP 监听器（真实 TCP socket，讲最小 SMTP 对话），再驱动生产路径
// newEmailProviderDialer + gomail DialAndSend 把邮件真正发过去，断言服务端
// 收到了 MAIL FROM / RCPT TO / DATA 正文。证明告警邮件确实能经真实 socket 投递，
// 而不仅是源码存在——这是 businessClosureReady 需要的运行时证据之一。
//
// 关键注意事项：本地监听器不广告 AUTH / STARTTLS，因此 gomail 会以明文发送且
// 跳过认证；dialer.SSL 关闭。仅用于 127.0.0.1 环回的投递语义验证，不代表生产
// 加密配置。
package service

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	gomail "gopkg.in/gomail.v2"
)

// capturedEmail 记录服务端在一次 SMTP 会话里观察到的关键字段。
type capturedEmail struct {
	mailFrom string
	rcptTo   []string
	data     string
}

// startMinimalSMTPServer 在 127.0.0.1 上起一个只接一次连接的最小 SMTP 服务端。
// 返回监听地址（host, port）和一个在会话结束后可读的结果通道。
func startMinimalSMTPServer(t *testing.T) (string, int, <-chan capturedEmail) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	result := make(chan capturedEmail, 1)

	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

		var cap capturedEmail
		r := bufio.NewReader(conn)
		w := bufio.NewWriter(conn)
		writeLine := func(s string) {
			fmt.Fprintf(w, "%s\r\n", s)
			_ = w.Flush()
		}

		writeLine("220 localhost ESMTP test")
		inData := false
		var body strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			trimmed := strings.TrimRight(line, "\r\n")

			if inData {
				if trimmed == "." {
					inData = false
					cap.data = body.String()
					writeLine("250 OK queued")
					continue
				}
				body.WriteString(trimmed)
				body.WriteString("\n")
				continue
			}

			upper := strings.ToUpper(trimmed)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				// 只广告基本能力，不声明 AUTH / STARTTLS，让 gomail 走明文无认证。
				writeLine("250-localhost")
				writeLine("250 SIZE 10485760")
			case strings.HasPrefix(upper, "MAIL FROM"):
				cap.mailFrom = extractAngleAddr(trimmed)
				writeLine("250 OK")
			case strings.HasPrefix(upper, "RCPT TO"):
				cap.rcptTo = append(cap.rcptTo, extractAngleAddr(trimmed))
				writeLine("250 OK")
			case upper == "DATA":
				inData = true
				writeLine("354 End data with <CR><LF>.<CR><LF>")
			case upper == "QUIT":
				writeLine("221 Bye")
				result <- cap
				return
			default:
				writeLine("250 OK")
			}
		}
	}()

	return "127.0.0.1", addr.Port, result
}

func extractAngleAddr(line string) string {
	start := strings.Index(line, "<")
	end := strings.Index(line, ">")
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return strings.TrimSpace(line)
}

// TestAlarmEmailRealSocketDelivery 是 REQ-23 的运行时投递证据：真实 socket、
// 生产 dialer、gomail DialAndSend，断言 SMTP 服务端收到了完整邮件。
func TestAlarmEmailRealSocketDelivery(t *testing.T) {
	host, port, result := startMinimalSMTPServer(t)

	emailConf := model.EmailConfig{
		Host:         host,
		Port:         port,
		FromEmail:    "alerts@aetherlink.test",
		FromPassword: "unused-on-localhost",
	}
	dialer := newEmailProviderDialer(emailConf)

	// 与生产 sendEmailMessageForDevices 一致的消息构造。
	m := gomail.NewMessage()
	m.SetHeader("From", emailConf.FromEmail)
	m.SetHeader("To", "ops@customer.test")
	m.SetHeader("Subject", "RDI Alarm: T2 over threshold")
	m.SetBody("text/plain", "Device pump-3 exceeded T2 threshold at 24.5C\n\n---\nThis email was sent by AetherLink IoT")

	// 走真实生产投递路径（含有界重试），不替换 sendMailWithDialer 缝。
	if err := deliverTenantAlarmEmail(dialer, m); err != nil {
		t.Fatalf("deliverTenantAlarmEmail over real socket failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var got capturedEmail
	go func() {
		defer wg.Done()
		select {
		case got = <-result:
		case <-time.After(10 * time.Second):
			t.Errorf("timed out waiting for SMTP server to capture the message")
		}
	}()
	wg.Wait()

	if got.mailFrom != "alerts@aetherlink.test" {
		t.Errorf("MAIL FROM = %q, want alerts@aetherlink.test", got.mailFrom)
	}
	if len(got.rcptTo) != 1 || got.rcptTo[0] != "ops@customer.test" {
		t.Errorf("RCPT TO = %v, want [ops@customer.test]", got.rcptTo)
	}
	if !strings.Contains(got.data, "T2 over threshold") {
		t.Errorf("DATA missing subject; got:\n%s", got.data)
	}
	if !strings.Contains(got.data, "exceeded T2 threshold") {
		t.Errorf("DATA missing body; got:\n%s", got.data)
	}
}
