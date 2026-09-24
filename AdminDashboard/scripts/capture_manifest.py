#!/usr/bin/env python3
"""Package actual review-browser images; require explicit CI source provenance."""
from pathlib import Path
from datetime import datetime,timezone
import hashlib,json,os,re,shutil,struct
root=Path(__file__).resolve().parents[1]
sha=os.environ.get('GITHUB_SHA','')
assert re.fullmatch('[0-9a-f]{40}',sha),'Exact CI source revision required'
source=root/'evidence/screens';out=root/'evidence/gallery';out.mkdir(parents=True,exist_ok=True)
images=out/'images';images.mkdir(exist_ok=True)
captures=[]
for p in sorted(source.glob('*.png')):
    data=p.read_bytes();assert data[:8]==b'\x89PNG\r\n\x1a\n';width,height=struct.unpack('>II',data[16:24]);shutil.copyfile(p,images/p.name)
    captures.append({'path':'images/'+p.name,'screen':p.stem.replace('-',' '),'source_commit':sha,'captured_at':datetime.fromtimestamp(p.stat().st_mtime,timezone.utc).isoformat(),'environment':'test','fixture_id':'admin-review-v0.7-public-synthetic','state':'synthetic runtime','viewport':{'width':390 if p.stem.endswith('mobile') else 1440,'height':844 if p.stem.endswith('mobile') else 1000,'capture_width':width,'capture_height':height},'sha256':hashlib.sha256(data).hexdigest()})
assert len(captures)>=35, 'Incomplete runtime capture set'
(out/'SCREENSHOTS.json').write_text(json.dumps({'schema_version':1,'service':'AdminDashboard','status':'captured','captures':captures},indent=2)+'\n')
text='# AdminDashboard runtime gallery\n\nActual Chromium captures from the separate synthetic review build. No live bank, customer, provider or notification activity. Authentication examples in review are deliberately public fixtures; real tokens and identity documents are never captured.\n\nSource revision: `'+sha+'`. Each image hash, time and viewport is recorded in SCREENSHOTS.json.\n'
for v in captures: text+='\n## '+v['screen']+'\n\n!['+v['screen']+' — synthetic runtime]('+v['path']+')\n'
(out/'SCREENSHOTS.md').write_text(text)
print('Packaged',len(captures),'actual browser screenshots from',sha)
