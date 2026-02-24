import csv, io

csv_path = r"D:\Programs\pilipiliworker\piliMinusB\issues\2026-02-24_14-05-00-pilminusb-migration.csv"

with open(csv_path, "r", encoding="utf-8-sig") as f:
    reader = csv.DictReader(f)
    fieldnames = reader.fieldnames
    rows = list(reader)

for r in rows:
    if r["id"] == "PMB-030":
        r["dev_state"] = "已完成"
        r["review_initial_state"] = "待验收"
        r["git_state"] = "已提交"
        r["notes"] = "model/favorite.go + handler/favorite.go + main.go routes registered; go build OK"
    if r["id"] == "PMB-040":
        r["dev_state"] = "已完成"
        r["review_initial_state"] = "待验收"
        r["git_state"] = "已提交"
        r["notes"] = "All 9 endpoints implemented in handler/favorite.go (same file as PMB-030)"

buf = io.StringIO()
writer = csv.DictWriter(buf, fieldnames=fieldnames, quoting=csv.QUOTE_ALL)
writer.writeheader()
writer.writerows(rows)

with open(csv_path, "w", encoding="utf-8-sig", newline="") as f:
    f.write(buf.getvalue())

print("CSV updated: PMB-030=已完成, PMB-040=已完成")
