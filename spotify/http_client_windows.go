//go:build windows

package spotify

import (
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
)

const spotifyWindowsInternetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func spotifySystemProxy(req *http.Request) (*url.URL, error) {
	enabled, err := spotifyReadWindowsDWORD("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil, nil
	}
	server, err := spotifyReadWindowsString("ProxyServer")
	if err != nil || strings.TrimSpace(server) == "" {
		return nil, nil
	}
	override, _ := spotifyReadWindowsString("ProxyOverride")
	if spotifyProxyBypassed(req.URL.Hostname(), override) {
		return nil, nil
	}
	rawProxy := spotifyWindowsProxyForScheme(server, req.URL.Scheme)
	return spotifyParseProxyURL(rawProxy), nil
}

func spotifyWindowsProxyForScheme(server string, scheme string) string {
	server = strings.TrimSpace(server)
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	var generic string
	for _, item := range strings.Split(server, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key, value, found := strings.Cut(item, "=")
		if !found {
			if generic == "" {
				generic = item
			}
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), scheme) {
			return strings.TrimSpace(value)
		}
	}
	if generic != "" {
		return generic
	}
	if scheme == "https" {
		for _, item := range strings.Split(server, ";") {
			key, value, found := strings.Cut(strings.TrimSpace(item), "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "http") {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func spotifyReadWindowsDWORD(name string) (uint32, error) {
	value, valueType, err := spotifyReadWindowsInternetSetting(name)
	if err != nil {
		return 0, err
	}
	if valueType != syscall.REG_DWORD || len(value) < 4 {
		return 0, errors.New("spotify Windows proxy setting is not a DWORD")
	}
	return binary.LittleEndian.Uint32(value[:4]), nil
}

func spotifyReadWindowsString(name string) (string, error) {
	value, valueType, err := spotifyReadWindowsInternetSetting(name)
	if err != nil {
		return "", err
	}
	if valueType != syscall.REG_SZ && valueType != syscall.REG_EXPAND_SZ {
		return "", errors.New("spotify Windows proxy setting is not a string")
	}
	if len(value)%2 != 0 {
		return "", errors.New("spotify Windows proxy setting has an invalid string size")
	}
	utf16Value := make([]uint16, len(value)/2)
	for index := range utf16Value {
		utf16Value[index] = binary.LittleEndian.Uint16(
			value[index*2 : index*2+2],
		)
	}
	result := syscall.UTF16ToString(utf16Value)
	if valueType == syscall.REG_EXPAND_SZ {
		result = os.ExpandEnv(result)
	}
	return result, nil
}

func spotifyReadWindowsInternetSetting(name string) ([]byte, uint32, error) {
	subkey, err := syscall.UTF16PtrFromString(spotifyWindowsInternetSettingsKey)
	if err != nil {
		return nil, 0, err
	}
	valueName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, 0, err
	}

	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(
		syscall.HKEY_CURRENT_USER,
		subkey,
		0,
		syscall.KEY_READ,
		&key,
	); err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = syscall.RegCloseKey(key)
	}()

	var valueType uint32
	var valueSize uint32
	if err := syscall.RegQueryValueEx(
		key,
		valueName,
		nil,
		&valueType,
		nil,
		&valueSize,
	); err != nil {
		return nil, 0, err
	}
	if valueSize == 0 {
		return nil, valueType, nil
	}
	value := make([]byte, valueSize)
	if err := syscall.RegQueryValueEx(
		key,
		valueName,
		nil,
		&valueType,
		&value[0],
		&valueSize,
	); err != nil {
		return nil, 0, err
	}
	return value[:valueSize], valueType, nil
}
