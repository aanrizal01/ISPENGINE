import sqlite3

con = sqlite3.connect('/home/anri01/gogiga-isp/onboarding.db')
cur = con.cursor()

print("=== SEBELUM PENGHAPUSAN ===")
print("--- Partners ---")
for r in cur.execute("SELECT id, code, name FROM partners").fetchall():
    print(r)

print("--- Staff Users ---")
for r in cur.execute("SELECT id, username, full_name, role FROM staff_users").fetchall():
    print(r)

# Hapus dari partners
cur.execute("DELETE FROM partners WHERE code IN ('ANDI-PYK', 'SITI-PYK', 'BUDI-TECH', 'ANDI', 'SITI', 'BUDI') OR name LIKE '%Andi%' OR name LIKE '%Siti%' OR name LIKE '%Budi%'")

# Hapus dari staff_users
cur.execute("DELETE FROM staff_users WHERE username IN ('andi_sales', 'budi_tech', 'siti', 'siti_sales') OR username LIKE 'test_staff%' OR full_name LIKE '%Andi Pratama%' OR full_name LIKE '%Budi Santoso%' OR full_name LIKE '%Siti Rahma%'")

con.commit()

print("\n=== SETELAH PENGHAPUSAN ===")
print("--- Partners ---")
for r in cur.execute("SELECT id, code, name FROM partners").fetchall():
    print(r)

print("--- Staff Users ---")
for r in cur.execute("SELECT id, username, full_name, role FROM staff_users").fetchall():
    print(r)

con.close()
