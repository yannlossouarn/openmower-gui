import {App, Button, Col, Modal, Row, Typography} from "antd";
import {useState} from "react";
import {fetchEventSource} from "@microsoft/fetch-event-source";
import {FormButtonGroup} from "@formily/antd-v5";
import {StyledTerminal} from "./StyledTerminal.tsx";
import Terminal, {ColorMode, TerminalOutput} from "react-terminal-ui";
import AsyncButton from "./AsyncButton.tsx";
import {useIsMobile} from "../hooks/useIsMobile";
import {useThemeMode} from "../theme/ThemeContext.tsx";

export const FlashGPSComponent = (props: { onNext: () => void, onPrevious: () => void }) => {
    const isMobile = useIsMobile();
    const {colors} = useThemeMode();
    const {notification} = App.useApp();
    const [data, setData] = useState<string[]>()
    const flashGPS = async () => {
        try {
            await fetchEventSource(`/api/setup/flashGPS`, {
                method: "POST",
                keepalive: false,
                headers: {
                    Accept: "text/event-stream",
                },
                onopen(res: Response) {
                    if (res.ok && res.status === 200) {
                        console.log("Connected to GPS flash log stream");
                    } else if (
                        res.status >= 400 &&
                        res.status < 500 &&
                        res.status !== 429
                    ) {
                        notification.error({
                            message: "Error retrieving log stream",
                            description: res.statusText,
                        });
                    }
                    setData([])
                    return Promise.resolve()
                },
                onmessage(event: {event: string; data: string}) {
                    if (event.event == "end") {
                        notification.success({
                            message: "GPS configuration flashed",
                            description: "The GPS hardware has been configured to match the protocol selected in Settings.",
                        });
                        setTimeout(() => {
                            props.onNext();
                        }, 10000);
                        return;
                    } else if (event.event == "error") {
                        notification.error({
                            message: "Error flashing GPS",
                            description: event.data,
                        });
                        return;
                    } else {
                        setData((prev: string[] | undefined) => [...(prev ?? []), event.data]);
                    }
                },
                onclose() {
                    notification.success({
                        message: "Log stream closed",
                    });
                },
                onerror(err: unknown) {
                    notification.error({
                        message: "Error retrieving log stream",
                        description: String(err),
                    });
                },
            });
        } catch (e: unknown) {
            notification.error({
                message: "Error flashing GPS",
                description: String(e),
            });
        }
    };
    return <Row>
        <Col span={24} style={{textAlign: "center"}}>
            <Typography.Title level={4}>
                Flash uBlox GPS Configuration
            </Typography.Title>
            <Typography.Paragraph>
                Click the button below to upload the GPS configuration file and set the output protocol to
                match <Typography.Text code>OM_GPS_PROTOCOL</Typography.Text> from your Settings.
            </Typography.Paragraph>
            <Typography.Paragraph type="secondary">
                Make sure you have selected the correct GPS protocol in the Settings step before continuing.
            </Typography.Paragraph>
            <Modal
                title="GPS configuration log"
                width={"70%"}
                open={(data && data.length > 0)}
                cancelButtonProps={{style: {display: "none"}}}
                onOk={() => {
                    setData([])
                }}
            >
                <StyledTerminal>
                    <Terminal colorMode={ColorMode.Dark}>
                        {(data ?? []).map((line: string, index: number) => {
                            return <TerminalOutput key={index}>{line}</TerminalOutput>;
                        })}
                    </Terminal>
                </StyledTerminal>
            </Modal>
        </Col>
        <Col span={24} style={{
            position: "fixed",
            bottom: isMobile ? 'calc(56px + env(safe-area-inset-bottom, 0px))' : 20,
            left: isMobile ? 0 : undefined,
            right: isMobile ? 0 : undefined,
            padding: isMobile ? '8px 12px' : undefined,
            background: isMobile ? colors.bgCard : undefined,
            borderTop: isMobile ? `1px solid ${colors.border}` : undefined,
            zIndex: 50,
        }}>
            <FormButtonGroup>
                <Button onClick={props.onPrevious}>Previous</Button>
                <AsyncButton type={"primary"} onAsyncClick={flashGPS}>Flash GPS Configuration</AsyncButton>
                <Button onClick={props.onNext}>Skip</Button>
            </FormButtonGroup>
        </Col>
    </Row>;
};
