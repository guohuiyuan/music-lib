// 汽水本机签名垫片：stdin 收 {url,method,body,headers,ts_ms}，
// 用官方客户端自带的 bdms.node 生成 x-helios / x-medusa，stdout 出 {"headers":{...}}。
// 由 music-lib/soda 的内置本机签名器调用（node -e 或客户端 Electron + ELECTRON_RUN_AS_NODE=1）。
// 思路参考 sodam / libresoda 的 mssdk-bridge（AGPL-3.0），此处为独立实现。
const fs = require('fs');

let input = '';
process.stdin.on('data', (chunk) => {
  input += chunk;
});

process.stdin.on('end', () => {
  try {
    const request = JSON.parse(input || '{}');
    const bdmsPath = String(process.env.QISHUI_BDMS_PATH || '').trim();
    if (!bdmsPath || !fs.existsSync(bdmsPath)) {
      throw new Error('找不到 bdms.node，请设置 QISHUI_BDMS_PATH 或 QISHUI_CLIENT_DIR');
    }
    const deviceId = String(process.env.QISHUI_DEVICE_ID || '').trim();
    if (!deviceId) {
      throw new Error('缺少 QISHUI_DEVICE_ID');
    }
    const bdms = require(bdmsPath);
    bdms.init({ deviceId: deviceId });

    const source = request.headers || {};
    const names = Object.keys(source).filter(
      (name) => name && source[name] !== undefined && source[name] !== null,
    );
    const headerLines = names
      .map((name) => name + '\r\n' + String(source[name]))
      .join('\r\n');

    const raw = String(
      bdms.generateHttpSignatureHeaders(String(request.url || ''), headerLines) || '',
    );
    const fields = raw.split('\r\n').filter((item) => item.trim());
    const headers = {};
    for (let index = 0; index + 1 < fields.length; index += 2) {
      headers[fields[index]] = fields[index + 1];
    }
    if (!headers['X-Helios'] || !headers['X-Medusa']) {
      throw new Error('签名不完整：收到 ' + Object.keys(headers).length + ' 个头');
    }
    process.stdout.write(JSON.stringify({ headers: headers }));
  } catch (error) {
    process.stderr.write(String((error && error.message) || error));
    process.exit(1);
  }
});
