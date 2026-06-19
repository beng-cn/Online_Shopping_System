import matplotlib.pyplot as plt
import numpy as np
import os

plt.rcParams['font.sans-serif'] = ['Microsoft YaHei']
plt.rcParams['axes.unicode_minus'] = False

# 修正后数据
x = [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
y = [32, 53, 78, 100, 124, 146, 170, 193, 216, 238, 261]

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
        label=f'线性拟合: y = {z[0]:.4f}x + {z[1]:.3f}  (R²={r2:.5f})', zorder=2)

# 标注数据点
for xi, yi in zip(x, y):
    ax.annotate(f'({xi}, {yi})', (xi, yi), textcoords='offset points',
                xytext=(0, -18), ha='center', fontsize=6.5, color='#555555')

ax.set_xlabel('电压 x (V)', fontsize=13, fontweight='bold')
ax.set_ylabel('频率 y (Hz)', fontsize=13, fontweight='bold')
ax.set_title('电压 — 频率 对应关系曲线', fontsize=17, fontweight='bold', pad=15)
ax.grid(True, linestyle='--', alpha=0.35)
ax.legend(fontsize=10.5, loc='upper left')
ax.set_xlim(1.2, 12.8)
ax.set_ylim(10, 290)

save_dir = r'C:\Users\LENOVO\Pictures\Camera Roll'
os.makedirs(save_dir, exist_ok=True)
save_path = os.path.join(save_dir, 'voltage_frequency_curve.png')
fig.savefig(save_path, dpi=150, bbox_inches='tight', facecolor='white', edgecolor='none')
plt.close()
print(f'已覆盖保存: {save_path}')
print(f'拟合结果: y = {z[0]:.4f}x + {z[1]:.3f}, R² = {r2:.5f}')
