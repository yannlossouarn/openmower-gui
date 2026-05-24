import {Collapse, Form, Input, InputNumber, Modal, Select, Space, Switch, Typography} from "antd";
import {MowingAreaEdit} from "../utils/types.ts";

const {Text} = Typography;

interface EditAreaModalProps {
    open: boolean;
    area: MowingAreaEdit;
    onChange: (area: MowingAreaEdit) => void;
    onSave: () => void;
    onCancel: () => void;
}

const AREA_TYPE_OPTIONS = [
    {value: 'workarea', label: 'Mowing Area'},
    {value: 'navigation', label: 'Navigation Area'},
    {value: 'obstacle', label: 'Obstacle'},
];

const RAD_TO_DEG = 180 / Math.PI;
const DEG_TO_RAD = Math.PI / 180;

/**
 * A toggle+input combo for an optional numeric override.
 * When the switch is off, the value is null (= use global default).
 */
function OverrideField({
    label,
    hint,
    value,
    onChange,
    min,
    max,
    step = 1,
    precision = 0,
    addonAfter,
    toDisplay,
    fromDisplay,
}: {
    label: string;
    hint?: string;
    value: number | null;
    onChange: (v: number | null) => void;
    min?: number;
    max?: number;
    step?: number;
    precision?: number;
    addonAfter?: string;
    /** Transform stored value → displayed value (e.g. rad→deg). */
    toDisplay?: (v: number) => number;
    /** Transform displayed value → stored value (e.g. deg→rad). */
    fromDisplay?: (v: number) => number;
}) {
    const enabled = value !== null;
    const displayVal = enabled && value !== null
        ? (toDisplay ? toDisplay(value) : value)
        : undefined;

    return (
        <Form.Item
            label={
                <Space size={8}>
                    <Switch
                        size="small"
                        checked={enabled}
                        onChange={(on) => {
                            if (!on) {
                                onChange(null);
                            } else {
                                // Initialise to a sensible default when enabling
                                const init = toDisplay ? 0 : 0;
                                onChange(fromDisplay ? fromDisplay(init) : init);
                            }
                        }}
                    />
                    <span>{label}</span>
                </Space>
            }
            style={{marginBottom: 8}}
        >
            {enabled ? (
                <InputNumber
                    min={min}
                    max={max}
                    step={step}
                    precision={precision}
                    value={displayVal}
                    addonAfter={addonAfter}
                    onChange={(v) => {
                        if (v === null || v === undefined) {
                            onChange(null);
                        } else {
                            onChange(fromDisplay ? fromDisplay(v) : v);
                        }
                    }}
                    style={{width: '100%'}}
                />
            ) : (
                <Text type="secondary" style={{fontSize: 12}}>
                    {hint ?? 'Using global default'}
                </Text>
            )}
        </Form.Item>
    );
}

export const EditAreaModal = ({open, area, onChange, onSave, onCancel}: EditAreaModalProps) => (
    <Modal
        open={open}
        title={area.name ? `Edit "${area.name}"` : "Edit area"}
        okText="Save"
        cancelText="Cancel"
        onOk={onSave}
        onCancel={onCancel}
        destroyOnClose
    >
        <Form layout="vertical" style={{marginTop: 16}}>
            <Form.Item label="Area type">
                <Select
                    value={area.feature_type}
                    onChange={(v) => onChange({...area, feature_type: v})}
                    options={AREA_TYPE_OPTIONS}
                />
            </Form.Item>
            {area.feature_type === 'workarea' && (
                <Form.Item label="Area name">
                    <Input
                        key="areaname"
                        placeholder="e.g. Front lawn"
                        value={area.name}
                        onChange={(e) => onChange({...area, name: e.target.value})}
                        autoFocus
                    />
                </Form.Item>
            )}
            {area.feature_type === 'workarea' && (
                <Form.Item label="Mowing order">
                    <InputNumber
                        key="mowingorder"
                        min={1}
                        value={area.mowing_order}
                        onChange={(v) => onChange({...area, mowing_order: v ?? 9999})}
                        style={{width: '100%'}}
                    />
                </Form.Item>
            )}
            {area.feature_type === 'workarea' && (
                <Collapse
                    size="small"
                    style={{marginTop: 8}}
                    items={[{
                        key: 'advanced',
                        label: 'Advanced — per-area overrides',
                        children: (
                            <>
                                <Text type="secondary" style={{fontSize: 12, display: 'block', marginBottom: 12}}>
                                    Toggle a parameter to override the global value for this area only.
                                    Leave it off to use the global mower_logic setting.
                                </Text>
                                <OverrideField
                                    label="Mowing angle"
                                    hint="Using global angle"
                                    value={area.angle}
                                    onChange={(v) => onChange({...area, angle: v})}
                                    min={-180}
                                    max={180}
                                    step={5}
                                    precision={1}
                                    addonAfter="°"
                                    toDisplay={(rad) => parseFloat((rad * RAD_TO_DEG).toFixed(1))}
                                    fromDisplay={(deg) => deg * DEG_TO_RAD}
                                />
                                <OverrideField
                                    label="Outline passes"
                                    hint="Using global outline count"
                                    value={area.outline_count}
                                    onChange={(v) => onChange({...area, outline_count: v})}
                                    min={0}
                                    max={20}
                                    step={1}
                                    precision={0}
                                />
                                <OverrideField
                                    label="Outline overlap passes"
                                    hint="Using global overlap count"
                                    value={area.outline_overlap_count}
                                    onChange={(v) => onChange({...area, outline_overlap_count: v})}
                                    min={0}
                                    max={20}
                                    step={1}
                                    precision={0}
                                />
                                <OverrideField
                                    label="Outline offset"
                                    hint="Using global outline offset"
                                    value={area.outline_offset}
                                    onChange={(v) => onChange({...area, outline_offset: v})}
                                    min={-2}
                                    max={2}
                                    step={0.01}
                                    precision={2}
                                    addonAfter="m"
                                />
                            </>
                        ),
                    }]}
                />
            )}
        </Form>
    </Modal>
);
