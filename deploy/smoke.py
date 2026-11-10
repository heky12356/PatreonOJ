"""Run deployment acceptance against the public site, without logging credentials.

Run on the deployment server. Creates the first administrator and an A+B sample
only when its private deployment state does not already record them.
"""
import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(os.environ.get('PATREONOJ_DEPLOY_ROOT', '/opt/patreonoj'))
ENV = ROOT / 'shared/compose.env'
STATE = ROOT / 'shared/admin-credentials.json'
COMPOSE = ROOT / 'current/PatreonOJ/deploy/compose.yaml'
SETTINGS = dict(line.split('=', 1) for line in ENV.read_text().splitlines() if '=' in line and not line.startswith('#'))
BASE = 'https://' + SETTINGS['SITE_DOMAIN']
ORIGIN = os.environ.get('PATREONOJ_SMOKE_ORIGIN', '').rstrip('/')


def save(state):
    STATE.write_text(json.dumps(state, indent=2, ensure_ascii=False) + '\n')
    STATE.chmod(0o600)


class Client:
    def __init__(self):
        self.cookie = ''

    def request(self, method, path, data=None, raw=False, cookie=None):
        url = path if path.startswith('http') else BASE + path
        headers = {'User-Agent': 'PatreonOJ-Deployment-Check/1.0'}
        if ORIGIN:
            parsed = urllib.parse.urlsplit(url)
            assert parsed.netloc == urllib.parse.urlsplit(BASE).netloc
            headers['Host'] = parsed.netloc
            url = ORIGIN + parsed.path + ('?' + parsed.query if parsed.query else '')
        token = self.cookie if cookie is None else cookie
        if token:
            headers['Cookie'] = token
        if isinstance(data, bytes):
            body = data
            headers['Content-Type'] = 'application/octet-stream'
        elif data is not None:
            body = json.dumps(data).encode()
            headers['Content-Type'] = 'application/json'
        else:
            body = None
        req = urllib.request.Request(url, body, headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=90) as response:
                payload = response.read()
                set_cookie = response.headers.get('Set-Cookie', '')
                if 'session_id=' in set_cookie:
                    self.cookie = set_cookie.split(';', 1)[0]
                    if method == 'POST' and path.endswith('/login'):
                        assert 'Secure' in set_cookie and 'HttpOnly' in set_cookie
                return payload if raw else json.loads(payload)
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode(errors='replace')
            # Presigned URL query strings and cookies are deliberately omitted.
            raise RuntimeError(f'{method} {urllib.parse.urlsplit(url).path}: HTTP {exc.code}: {detail[:500]}') from None


def data(payload):
    for key in ('data', 'result'):
        if key in payload:
            return payload[key]
    return payload


def upload(client, prefix, filename, content):
    result = data(client.request('GET', '/api/oss/upload-url?' + urllib.parse.urlencode({'path': prefix, 'filename': filename})))
    assert result['url'].startswith(BASE + '/oss/')
    client.request('PUT', result['url'], content, raw=True)
    return result


def main():
    os.umask(0o077)
    client = Client()
    assert client.request('GET', '/api/healthz')['status'] == 'ok'
    assert b'<html' in client.request('GET', '/', raw=True).lower()
    print('PASS ' + ('origin' if ORIGIN else 'public') + ' homepage and API health', flush=True)
    state = json.loads(STATE.read_text()) if STATE.exists() else {'username': 'admin', 'password': secrets.token_urlsafe(24)}
    save(state)
    if not state.get('registered'):
        try:
            client.request('POST', '/api/user/register', {'username': state['username'], 'password': state['password']})
        except RuntimeError as exc:
            if 'HTTP 409' not in str(exc):
                raise
            # Prove this is our account before granting any privileges. A name
            # collision must never promote somebody else's existing account.
            client.request('POST', '/api/user/login', {'username': state['username'], 'password': state['password']})
        subprocess.run(['docker', 'compose', '--env-file', str(ENV), '-f', str(COMPOSE), 'exec', '-T', 'api', '/app/patreonoj-admin', '-username', state['username']], check=True, capture_output=True)
        state['registered'] = True
        save(state)
    login = data(client.request('POST', '/api/user/login', {'username': state['username'], 'password': state['password']}))
    assert 'admin' in login['permissions']
    profile = data(client.request('GET', '/api/user/me'))
    uid = profile['uuid']
    assert data(client.request('GET', '/api/user/' + uid))['uuid'] == uid
    client.request('PUT', '/api/user/' + uid, {'nickname': profile.get('nickname') or '管理员'})
    public = Client().request('GET', f"/api/user/{profile['id']}/public")
    assert 'email' not in data(public)
    print('PASS administrator login, profile, UUID update and public privacy', flush=True)

    png = base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6cdsAAAAASUVORK5CYII=')
    avatar = upload(client, f'avatars/{uid}/', 'deployment-check.png', png)
    public_url = f"{BASE}/oss/{avatar['bucket']}/{avatar['key']}"
    assert client.request('GET', public_url, raw=True) == png
    print('PASS public-domain signed upload and avatar read', flush=True)

    if not state.get('question_pk'):
        question = data(client.request('POST', '/api/question/', {
            'question_id': '1000', 'title': 'A + B',
            'content': '## 题目描述\n输入两个整数 a 和 b，输出它们的和。\n\n## 输入格式\n一行，包含两个整数，以空格分隔。\n\n## 输出格式\n输出 a + b。\n\n## 样例\n输入：`1 2`\n\n输出：`3`\n\n## 数据范围\n-1000000 ≤ a, b ≤ 1000000。',
            'difficulty': '简单', 'tags': '基础,输入输出', 'status': 'published', 'time_limit': 2000, 'memory_limit': 256,
        }))
        state['question_pk'] = question['id']
        save(state)
    qid = state['question_pk']
    existing = data(client.request('GET', f'/api/testcase/question/{qid}'))
    cases = existing if isinstance(existing, list) else existing.get('result', [])
    if not cases:
        for number, stdin, stdout, hidden in [(1, '1 2\n', '3\n', False), (2, '5 7\n', '12\n', True)]:
            inp = upload(client, f'problems/{qid}/', f'{number}.in', stdin.encode())
            out = upload(client, f'problems/{qid}/', f'{number}.out', stdout.encode())
            testcase = data(client.request('POST', '/api/testcase/oss/commit', {'question_pk': qid, 'input_key': inp['key'], 'output_key': out['key'], 'is_hidden': hidden}))
            cases.append(testcase)
    assert len(cases) >= 2
    print('PASS sample question and public/hidden OSS testcases', flush=True)

    sources = {
        'python': 'a,b=map(int,input().split())\nprint(a+b)\n',
        'cpp': '#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<"\\n";}\n',
        'java': 'public class Main { public static void main(String[] args) { java.util.Scanner s=new java.util.Scanner(System.in); long a=s.nextLong(),b=s.nextLong(); System.out.println(a+b); } }\n',
        'go': 'package main\nimport "fmt"\nfunc main(){var a,b int;fmt.Scan(&a,&b);fmt.Println(a+b)}\n',
        'wrong_python': 'print(0)\n',
    }
    submission_ids = []
    for name, source in sources.items():
        language = 'python' if name == 'wrong_python' else name
        submitted = data(client.request('POST', '/api/submission/', {'question_pk': qid, 'language': language, 'code': source}))
        sid = submitted['submission_id']
        submission_ids.append(sid)
        deadline = time.monotonic() + 180
        while True:
            result = data(client.request('GET', '/api/submission/' + sid))
            if result['status'] not in ('pending', 'processing'):
                break
            if time.monotonic() >= deadline:
                raise RuntimeError(f'{name} judgement timed out')
            time.sleep(1)
        assert result['status'] == 'completed', f"{name}: {result.get('error_code')} {result.get('error_message')}"
        expected_rate = 0 if name == 'wrong_python' else 1
        assert result['pass_rate'] == expected_rate, f"{name}: unexpected pass rate {result['pass_rate']}"
        for case in result['results']:
            if case.get('is_hidden'):
                assert not any(case.get(key) for key in ('input', 'expected_output', 'actual_output'))
        print(f'PASS {name}: {result["passed_cases"]}/{result["total_cases"]}', flush=True)
    state['verified_submissions'] = submission_ids
    save(state)
    old_cookie = client.cookie
    client.request('POST', '/api/user/logout')
    try:
        client.request('GET', '/api/user/me', cookie=old_cookie)
    except RuntimeError as exc:
        assert 'HTTP 401' in str(exc)
    else:
        raise AssertionError('revoked session still authorizes requests')
    print('PASS server session revoked', flush=True)
    print(f'Acceptance passed. Administrator credentials: {STATE}', flush=True)


if __name__ == '__main__':
    main()
