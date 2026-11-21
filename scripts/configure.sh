#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 - <<'PY'
import os,secrets,pathlib,shlex
p=pathlib.Path('.env')
data_dir=os.path.abspath(os.environ.get('FRIDAY_DATA_DIR','.data'))
if p.exists():
 text=p.read_text()
 if 'FRIDAY_DATA_DIR=' not in text:
  with p.open('a') as f: f.write(f'\nFRIDAY_DATA_DIR={shlex.quote(data_dir)}\n')
  print('Added FRIDAY_DATA_DIR to existing .env')
 else:
  print('Keeping existing .env')
else:
 password=secrets.token_hex(24)
 http_port=os.environ.get('AGENTFLOW_HTTP_PORT','8090');mysql_port=os.environ.get('FRIDAY_MYSQL_PORT','33315')
 data={'FRIDAY_DB_PASSWORD':password,'FRIDAY_DB_ROOT_PASSWORD':secrets.token_hex(24),'FRIDAY_MYSQL_PORT':mysql_port,'COMPOSE_PROJECT_NAME':os.environ.get('COMPOSE_PROJECT_NAME','friday'),'AGENTFLOW_HTTP_PORT':http_port,'AGENTFLOW_MYSQL_DSN':f'friday:{password}@tcp(127.0.0.1:{mysql_port})/friday','AGENTFLOW_SESSION_SECRET':secrets.token_hex(32),'AGENTFLOW_AUTH_REQUIRED':'true','FRIDAY_DATA_DIR':data_dir,'FRIDAY_BIND':os.environ.get('FRIDAY_BIND','0.0.0.0'),'FRIDAY_PUBLIC_URL':os.environ.get('FRIDAY_PUBLIC_URL',''),'FRIDAY_IMAGE':os.environ.get('FRIDAY_IMAGE',''),'AGENTFLOW_GITHUB_CLIENT_ID':os.environ.get('AGENTFLOW_GITHUB_CLIENT_ID','')}
 fd=os.open(str(p),os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'w') as f: f.write(''.join(f'{k}={shlex.quote(v)}\n' for k,v in data.items()))
 print('Created .env with unique local secrets')
PY
