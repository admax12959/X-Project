// One row's piece of the lineage graph (see lineageGraph.ts for the layout). The picture is
// decoration (aria-hidden): the status stays in its own column as icon + text. Meaning in the
// graph is carried by shape, not colour: mainline = diamond, Active = filled circle,
// Frozen = filled square, Retired = hollow circle, a line without versions = dashed ring.
import type { GraphRow } from './lineageGraph.ts';

export const LANE_W = 14;
const PAD = 10; // space left of lane 0
/** Height of a table row in px, border included. The rows set it themselves, so the graph of one row meets the next. */
export const ROW_H = 35;

/** Width of the graph column for a tree whose deepest lane is `lanes`. */
export const graphWidth = (lanes: number) => PAD + lanes * LANE_W + PAD;

// +0.5: a 1px-wide line centred on a pixel boundary would be blurred over two pixels.
const px = (x: number) => PAD + x * LANE_W + 0.5;
// Rails are drawn 1px past the row's edges so neighbouring rows join without a seam.
const py = (y: number) => y * ROW_H + (y === 0 ? -1 : y === 1 ? 1 : 0.5);

export function LineageGraphCell({ row, lanes, selected }: { row: GraphRow; lanes: number; selected: boolean }) {
  const cx = px(row.lane);
  const cy = ROW_H / 2 + 0.5;
  const ink = selected ? 'var(--color-accent)' : 'var(--color-text-3)';
  const { kind, empty } = row.dot;
  const hollow = empty || kind === 'retired';
  const common = { stroke: ink, strokeWidth: 1.5, fill: hollow ? '#fff' : ink, strokeDasharray: empty ? '2 1.5' : undefined };
  return (
    <svg width={graphWidth(lanes)} height={ROW_H} className="pointer-events-none absolute top-0 left-0 overflow-visible" aria-hidden>
      {row.segments.map((s, i) => (
        <line key={i} x1={px(s.x1)} y1={py(s.y1)} x2={px(s.x2)} y2={py(s.y2)} stroke="var(--color-text-3)" strokeWidth={1} />
      ))}
      {kind === 'main' && <rect x={cx - 4.5} y={cy - 4.5} width={9} height={9} transform={`rotate(45 ${cx} ${cy})`} {...common} />}
      {kind === 'frozen' && !empty && <rect x={cx - 3.5} y={cy - 3.5} width={7} height={7} {...common} />}
      {kind !== 'main' && !(kind === 'frozen' && !empty) && <circle cx={cx} cy={cy} r={4} {...common} />}
    </svg>
  );
}
