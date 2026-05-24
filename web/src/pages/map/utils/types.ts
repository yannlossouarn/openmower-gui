export class MowingAreaEdit {
    id?: string;
    name: string;
    mowing_order: number;
    orig_mowing_order: number;
    feature_type: string;
    orig_feature_type: string;
    index: number;
    /** Per-area override parameters. null = use global default. */
    angle: number | null;
    outline_count: number | null;
    outline_overlap_count: number | null;
    outline_offset: number | null;

    constructor() {
        this.name = '';
        this.mowing_order = 9999;
        this.orig_mowing_order = 9999;
        this.feature_type = 'workarea';
        this.orig_feature_type = 'workarea';
        this.index = -1;
        this.angle = null;
        this.outline_count = null;
        this.outline_overlap_count = null;
        this.outline_offset = null;
    }
}

export interface AreaListItem {
    id: string;
    name: string;
    ftype: string;
    areaLabel: string;
    mowingOrder?: number;
}
