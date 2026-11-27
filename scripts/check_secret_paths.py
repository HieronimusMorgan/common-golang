"""Basic tracked-path guard, NOT a secret content scanner."""
import sys
from pathlib import PurePosixPath
bad=[]
for raw in sys.stdin.buffer.read().split(b'\0'):
    if not raw: continue
    path=PurePosixPath(raw.decode('utf-8',errors='replace'))
    name=path.name.lower()
    if (name=='.env' or (name.startswith('.env.') and name!='.env.example') or name.endswith(('.pem','.key','.p12','.pfx')) or 'keys' in path.parts):
        bad.append(str(path))
if bad:
    sys.exit('Prohibited tracked credential paths: '+', '.join(bad))
