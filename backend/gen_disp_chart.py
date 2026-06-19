import matplotlib.pyplot as plt
import numpy as np
import os

plt.rcParams['font.sans-serif'] = ['Microsoft YaHei']
plt.rcParams['axes.unicode_minus'] = False

# 数据
x = [-1, -0.8, -0.6, -0.4, -0.2, 0, 0.2, 0.4, 0.6, 0.8, 1]
y = [-1120, -900, -676, -440, -220, 0, 220, 450, 670, 910, 1140]

fig, ax = plt.subplots(figsize=(11, 7), dpi=150)

# 数据点和连线
ax.plot(x, y, 'o-', color='#2E86AB', linewidth=2.2, markersize=9,
        markerfacecolor='#D64045', markeredgewidth=1.5, markeredgecolor='#A63232',
        label='实测数据点', zorder=3)

# 线性拟合
z = np.polyfit(x, y, 1)
p = np.poly1d(z)
ss_res = sum((yi - p(xi))**2 for xi, yi in zip(x, y))
ss_tot = sum((yi - np.mean(y))**2 for yi in y)
r2 = 1 - ss_res / ss_tot

ax.plot(x, p(x), '--', color='#A23B72', linewidth=1.5, alpha=0.75,
        label=f'线性拟合: Vo = {z[0]:.2f}*X + {z[1]:.1f}  (R²={r2:.5f})', zorder=2)

# 标注数据点
for xi, yi in zip(x, y):
    offset = -20 if yi >= 0 else 15
    ax.annotate(f'({xi:.1f}, {yi})', (xi, yi), textcoords='offset points',
                xytext=(0, offset), ha='center', fontsize=6.5, color='#555555')

ax.axhline(y=0, color='gray', linewidth=0.8, alpha=0.5)
ax.axvline(x=0, color='gray', linewidth=0.8, alpha=0.5)
ax.set_xlabel('位移 X (mm)', fontsize=13, fontweight='bold')
ax.set_ylabel('输出电压 Vo (mV)', fontsize=13, fontweight='bold')
ax.set_title('位移 — 输出电压 对应关系曲线', fontsize=17, fontweight='bold', pad=15)
ax.grid(True, linestyle='--', alpha=0.35)
ax.legend(fontsize=10.5, loc='upper left')
ax.set_xlim(-1.15, 1.15)
ax.set_ylim(-1300, 1350)

save_dir = r'C:\Users\LENOVO\Pictures\Camera Roll'
os.makedirs(save_dir, exist_ok=True)
save_path = os.path.join(save_dir, 'displacement_voltage_curve.png')
fig.savefig(save_path, dpi=150, bbox_inches='tight', facecolor='white', edgecolor='none')
plt.close()
print(f'已保存: {save_path}')
print(f'拟合结果: Vo = {z[0]:.2f}*X + {z[1]:.1f}, R^2 = {r2:.5f}')
