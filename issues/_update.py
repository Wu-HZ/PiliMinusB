import csv, io

csv_path = r"D:\Programs\pilipiliworker\piliMinusB\issues\2026-02-24_14-05-00-pilminusb-migration.csv"

with open(csv_path, "r", encoding="utf-8-sig") as f:
    reader = csv.DictReader(f)
    fieldnames = reader.fieldnames
    rows = list(reader)

for r in rows:
    if r["id"] == "PMB-080":
        r["dev_state"] = "已完成"
        r["review_initial_state"] = "待验收"
        r["git_state"] = "已提交"
        r["notes"] = "follow.dart(1) + member.dart(7) + video.dart(4) + fav.dart(1) migrated; server PgcSuccess for result envelope"

buf = io.StringIO()
writer = csv.DictWriter(buf, fieldnames=fieldnames, quoting=csv.QUOTE_ALL)
writer.writeheader()
writer.writerows(rows)

with open(csv_path, "w", encoding="utf-8-sig", newline="") as f:
    f.write(buf.getvalue())

print("CSV updated: PMB-080=已完成")
