/**
 * 「这个账号关联了 pic.bi」的徽章。
 *
 * 形状照 GroupBadge：同一张表里两种徽章挨着放，边框、圆角、字号不一样会
 * 读成两套东西。颜色单独挑了靛蓝——teal 在这套后台里已经被「正常/已验证」
 * 占住，amber 是管理员，再借用会让「关联」看起来像一种状态好坏。
 *
 * 只在关联时渲染：没关联是绝大多数行的常态，给每一行都印一个「未关联」
 * 是把噪声铺满整张表。调用点用 `{u.picbi_connected && <PicbiBadge />}`。
 */
export default function PicbiBadge({ compact = false }: { compact?: boolean }) {
  return (
    <span
      title="已关联 pic.bi 账号，AI 生成可扣 pic.bi 积分"
      className="inline-flex items-center gap-1 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-2 py-0.5 text-[10px] font-medium text-indigo-300"
    >
      <i className="fa-solid fa-link text-[9px]" />
      {compact ? "pic.bi" : "已关联 pic.bi"}
    </span>
  );
}
