import json, urllib.request

data = json.dumps({"url":"https://cn.wsj.com/articles/chinamanufacturing-gauge-shows-slower-growth-in-activity-for-june-f0354616?mod=cn_china"}).encode()
req = urllib.request.Request("http://127.0.0.1:8081/fetch/js", data=data, headers={"Content-Type":"application/json"})
resp = json.loads(urllib.request.urlopen(req, timeout=120).read())
print("Title:", resp.get("title","N/A"))
print("Paras:", len(resp.get("paragraphs",[])))
print("Source:", resp.get("source","N/A"))
print("Latency:", resp.get("latency_ms"),"ms")
