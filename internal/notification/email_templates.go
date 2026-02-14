package notification

import (
	"bytes"
	"fmt"
	"html/template"
)

// Email Templates
const (
	TemplateVerification   = "email_verification"
	TemplateForgotPassword = "forgot_password"
	TemplateSecurityCode   = "security_code"
)

// Base URL for assets (should be configurable via env, hardcoded for now or use app URL)
const AssetsBaseURL = "https://sapliy.com"

func GetEmailSubject(templateID string) string {
	switch templateID {
	case TemplateVerification:
		return "Verify your email address"
	case TemplateForgotPassword:
		return "Reset your password"
	case TemplateSecurityCode:
		return "Your security code"
	default:
		return "Notification from Sapliy"
	}
}

// Common styles and layout
const baseLayout = `
<!DOCTYPE html>
<html>
<head>
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <style>
        body { background-color: #f6f9fc; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; -webkit-font-smoothing: antialiased; font-size: 16px; line-height: 1.5; margin: 0; padding: 0; -ms-text-size-adjust: 100%; -webkit-text-size-adjust: 100%; }
        table { border-collapse: separate; mso-table-lspace: 0pt; mso-table-rspace: 0pt; width: 100%; }
        table td { font-family: sans-serif; font-size: 16px; vertical-align: top; }
        .container { display: block; margin: 0 auto !important; max-width: 580px; padding: 10px; width: 580px; }
        .content { box-sizing: border-box; display: block; margin: 0 auto; max-width: 580px; padding: 10px; }
        .main { background: #ffffff; border-radius: 8px; width: 100%; border: 1px solid #e1e9ee; }
        .wrapper { box-sizing: border-box; padding: 20px; }
        .header { padding: 24px 0; text-align: center; }
        .footer { clear: both; margin-top: 10px; text-align: center; width: 100%; }
        .footer td, .footer p, .footer span, .footer a { color: #8898aa; font-size: 12px; text-align: center; }
        h1 { font-size: 24px; font-weight: 700; margin: 0 0 20px 0; color: #32325d; text-align: center; }
        p { margin: 0 0 16px 0; color: #525f7f; }
        .btn { box-sizing: border-box; width: 100%; }
        .btn > tbody > tr > td { padding-bottom: 24px; }
        .btn table { width: auto; }
        .btn table td { background-color: #ffffff; border-radius: 4px; text-align: center; }
        .btn a { background-color: #5e6ad2; border: solid 1px #5e6ad2; border-radius: 4px; box-sizing: border-box; color: #ffffff; cursor: pointer; display: inline-block; font-size: 16px; font-weight: bold; margin: 0; padding: 12px 25px; text-decoration: none; text-transform: capitalize; }
        .logo { width: 120px; height: auto; }
        .code { background: #f6f9fc; border-radius: 4px; color: #32325d; font-family: monospace; font-size: 28px; letter-spacing: 4px; margin: 16px 0; padding: 16px; text-align: center; font-weight: 700; }
        .hr { border-top: 1px solid #e1e9ee; margin: 24px 0; }
    </style>
</head>
<body>
    <table role="presentation" border="0" cellpadding="0" cellspacing="0" class="body">
        <tr>
            <td>&nbsp;</td>
            <td class="container">
                <div class="header">
                    <img src="{{.LogoURL}}" alt="Sapliy Logo" class="logo" />
                </div>
                <div class="content">
                    <table role="presentation" class="main">
                        <tr>
                            <td class="wrapper">
                                {{.Content}}
                            </td>
                        </tr>
                    </table>
                    <div class="footer">
                        <table role="presentation" border="0" cellpadding="0" cellspacing="0">
                            <tr>
                                <td class="content-block">
                                    <span class="apple-link">Sapliy Fintech, Inc. 123 Innovation Dr, Tech City</span>
                                    <br> Don't want these emails? <a href="#">Unsubscribe</a>.
                                </td>
                            </tr>
                        </table>
                    </div>
                </div>
            </td>
            <td>&nbsp;</td>
        </tr>
    </table>
</body>
</html>
`

const verificationContent = `
    <h1>Verify Your Email</h1>
    <p>Thanks for signing up for Sapliy! We're excited to have you on board.</p>
    <p>Please confirm your account by clicking the button below:</p>
    <table role="presentation" border="0" cellpadding="0" cellspacing="0" class="btn btn-primary">
        <tbody>
            <tr>
                <td align="center">
                    <table role="presentation" border="0" cellpadding="0" cellspacing="0">
                        <tbody>
                            <tr>
                                <td> <a href="{{.Link}}" target="_blank">Verify Email Address</a> </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
        </tbody>
    </table>
    <p>This link will expire in 24 hours.</p>
`

const forgotPasswordContent = `
    <h1>Reset Your Password</h1>
    <p>You recently requested to reset your password for your Sapliy account.</p>
    <p>Click the button below to reset it:</p>
    <table role="presentation" border="0" cellpadding="0" cellspacing="0" class="btn btn-primary">
        <tbody>
            <tr>
                <td align="center">
                    <table role="presentation" border="0" cellpadding="0" cellspacing="0">
                        <tbody>
                            <tr>
                                <td> <a href="{{.Link}}" target="_blank">Reset Password</a> </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
        </tbody>
    </table>
    <p>If you didn't request a password reset, you can safely ignore this email.</p>
`

const securityCodeContent = `
    <h1>Security Code</h1>
    <p>Here is your security code for authentication:</p>
    <div class="code">{{.Code}}</div>
    <p>This code will expire in 10 minutes.</p>
    <p>Do not share this code with anyone.</p>
`

func RenderEmailTemplate(templateID string, data map[string]string) (string, error) {
	// Basic data enrichment
	tmplData := map[string]interface{}{
		"LogoURL": AssetsBaseURL + "/sapliy-logo.png", // Assumes mapped/hosted
	}
	for k, v := range data {
		tmplData[k] = v
	}

	var contentTmpl string
	switch templateID {
	case TemplateVerification:
		contentTmpl = verificationContent
	case TemplateForgotPassword:
		contentTmpl = forgotPasswordContent
	case TemplateSecurityCode:
		contentTmpl = securityCodeContent
	default:
		return "", fmt.Errorf("unknown template: %s", templateID)
	}

	// First render the content block
	tContent, err := template.New("content").Parse(contentTmpl)
	if err != nil {
		return "", err
	}
	var contentBuf bytes.Buffer
	if err := tContent.Execute(&contentBuf, tmplData); err != nil {
		return "", err
	}

	// Then inject content into base layout
	// Note: We use template.HTML to prevent escaping of the rendered content block
	tmplData["Content"] = template.HTML(contentBuf.String())

	tLayout, err := template.New("layout").Parse(baseLayout)
	if err != nil {
		return "", err
	}
	var layoutBuf bytes.Buffer
	if err := tLayout.Execute(&layoutBuf, tmplData); err != nil {
		return "", err
	}

	return layoutBuf.String(), nil
}
