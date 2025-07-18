package aliyun

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
)

type AliyunSMSClient struct {
	client       *dysmsapi.Client
	signName     string
	templateCode string
}

func NewAliyunSMSClient(
	region, accessKeyID, accessKeySecret, signName, templateCode string,
) (out.SMSClient, error) {
	client, err := dysmsapi.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("初始化 Aliyun 短信客户端失败: %w", err)
	}
	return &AliyunSMSClient{client: client, signName: signName, templateCode: templateCode}, nil
}

func (a *AliyunSMSClient) Send(ctx context.Context, phone string, code string) error {
	req := dysmsapi.CreateSendSmsRequest()
	req.Scheme = "https"
	req.PhoneNumbers = phone
	req.SignName = a.signName
	req.TemplateCode = a.templateCode
	req.TemplateParam = fmt.Sprintf(`{"code":"%s"}`, code)

	resp, err := a.client.SendSms(req)
	if err != nil {
		return fmt.Errorf("阿里云短信发送错误: %w", err)
	}
	if resp.Code != "OK" {
		return fmt.Errorf("阿里云短信发送失败: %s - %s", resp.Code, resp.Message)
	}
	return nil
}
