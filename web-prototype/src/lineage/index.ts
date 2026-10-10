// PROTOTYPE: three structurally different 分支谱系视图, switched via ?variant=A|B|C on /apps/:key.
import type { ComponentType } from 'react';
import type { LineageProps } from './LineDetail';
import { VariantA } from './VariantA';
import { VariantB } from './VariantB';
import { VariantC } from './VariantC';

export const VARIANTS: Record<string, { name: string; Component: ComponentType<LineageProps> }> = {
  A: { name: '泳道时间轴', Component: VariantA },
  B: { name: '按源版本缩进', Component: VariantB },
  C: { name: '表格 + 详情栏', Component: VariantC },
};
