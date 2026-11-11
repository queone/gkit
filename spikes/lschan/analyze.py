"""Score Wi-Fi channel crowding per scan from the JSON lines that main.go prints.

Usage: python3 analyze.py scans-10s-gap.jsonl

Interference for a candidate channel is the sum of each network's power in mW,
10^(RSSI/10), scaled by how much of its spectrum overlaps the candidate.
own() hard-codes the connected router from the recorded runs (channel 149,
80 MHz, about -43 dBm); change it to match the router you test against.
"""
import json, math, sys
rows=[json.loads(l) for l in open(sys.argv[1])]
W={1:20,2:40,3:80,4:160}
def span(b,c,w):
    k=W.get(w,20)//20
    if b==1:  # 2.4 GHz: MHz span around center freq
        f=2407+5*c; lo,hi=f-10,f+10
        if k>1: hi=f+30
        return lo,hi
    if b==2:
        base=36 if c<100 else (100 if c<149 else 149)
        f0=lambda ch:5000+5*ch
    else:
        base=1; f0=lambda ch:5950+5*ch
    s=base+((c-base)//(4*k))*4*k
    return f0(s)-10, f0(s)-10+20*k
def ov(a,b): return max(0,min(a[1],b[1])-max(a[0],b[0]))
cands={1:[(1,1),(6,1),(11,1)],2:[(c,3) for c in (36,52,100,116,132,149)],3:[(c,3) for c in range(1,234,16)]}
scans=sorted({r['scan'] for r in rows})
own=lambda r:r['band']==2 and r['channel']==149 and r['width']==3 and abs(r['rssi']+43)<=6
res={}
for s in scans:
    rs=[r for r in rows if r['scan']==s]
    print(f"scan {s}: {len(rs)} nets, own-AP lookalikes={sum(own(r) for r in rs)}")
    for b,cs in cands.items():
        for c,w in cs:
            cs_=span(b,c,w); p=0.0
            for r in rs:
                if r['band']!=b or own(r): continue
                sp=span(r['band'],r['channel'],r['width'])
                p+=10**(r['rssi']/10)*ov(sp,cs_)/(sp[1]-sp[0])
            res.setdefault((b,c),[]).append(10*math.log10(p) if p>0 else -120)
print()
name={1:'2.4',2:'5',3:'6'}
for b in (1,2,3):
    print(f"band {name[b]} GHz  candidate: per-scan interference dBm  (range)")
    for (bb,c),v in res.items():
        if bb!=b: continue
        if b==3 and max(v)<=-120: continue
        print(f"  ch {c:>3}: "+" ".join(f"{x:6.1f}" for x in v)+f"   range {max(v)-min(v):5.1f} dB")
    best=[min(((res[(b,c)][i],c) for (bb,c) in res if bb==b))[1] for i in range(len(scans))]
    print(f"  best per scan: {best}")
