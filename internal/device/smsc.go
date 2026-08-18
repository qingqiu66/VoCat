package device

import "context"

// ReadNativeQMIATSMSCenter returns the raw AT+CSCA? response text for the
// modem's SIM. It is used by the native OpenStick 410 VoWiFi adapter so IMS
// SMS submit can populate the RP service-centre address from the SIM itself.
func (manager *Manager) ReadNativeQMIATSMSCenter(ctx context.Context, id string) (string, error) {
	response, err := manager.ExecuteAT(ctx, id, "AT+CSCA?")
	if err != nil {
		return "", err
	}
	return response.Text(), nil
}
