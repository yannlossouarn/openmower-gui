import {App, Col, Row, Select, Typography, Button, Space} from "antd";
import {useEffect, useLayoutEffect, useRef, useState} from "react";
import Terminal, {ColorMode, TerminalOutput} from "react-terminal-ui";
import AsyncButton from "../components/AsyncButton.tsx";
import {useWS} from "../hooks/useWS.ts";
import {useApi} from "../hooks/useApi.ts";
import {StyledTerminal} from "../components/StyledTerminal.tsx";
import ansiHTML from "../utils/ansi.ts";
import {MowerActions} from "../components/MowerActions.tsx";

type ContainerList = { value: string, label: string, status: "started" | "stopped", labels: Record<string, string> };

export const LogsPage = () => {
    const guiApi = useApi();
    const {notification} = App.useApp();
    const [containers, setContainers] = useState<ContainerList[]>([]);
    const [containerId, setContainerId] = useState<string | undefined>(undefined);
    const [data, setData] = useState<string[]>([]);

    // --- Autoscroll state/refs ---
    const containerRef = useRef<HTMLDivElement | null>(null);
    const bottomRef = useRef<HTMLDivElement | null>(null);
    const atBottomRef = useRef<boolean>(true);
    const [autoFollow, setAutoFollow] = useState<boolean>(true);
    const THRESHOLD_PX = 60;

    // WS stream
    const stream = useWS<string>(() => {
        notification.error({ message: "Logs stream closed" });
    }, () => {
        // connected
        // keep current follow state
    }, (e, first) => {
        setData((prev) => (first ? [e] : [...prev, e]));
    });

    async function listContainers() {
        try {
            const containers = await guiApi.containers.containersList();
            if (containers.error) throw new Error(containers.error.error);

            const options = containers.data.containers?.flatMap<ContainerList>((container) => {
                if (!container.names || !container.id) return [];
                return [{
                    label: container.labels?.app ? `${container.labels.app} ( ${container.names[0].replace("/", "")} )` : container.names[0].replace("/", ""),
                    value: container.id,
                    status: container.state == "running" ? "started" : "stopped",
                    labels: container.labels ?? {}
                }];
            }) ?? [];

            setContainers(options);
            if (options.length && !containerId) setContainerId(options[0].value);
        } catch (e: any) {
            notification.error({
                message: "Failed to list containers",
                description: e.message
            });
        }
    }

    useEffect(() => {
        (async () => { await listContainers(); })();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    useEffect(() => {
        if (containerId) {
            // Reset logs when switching container to avoid mixing
            setData([]);
            // Re-enable follow and jump to bottom for the new stream
            setAutoFollow(true);
            atBottomRef.current = true;

            stream.start(`/api/containers/${containerId}/logs`);
            return () => { stream?.stop(); };
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [containerId]);

    const commandContainer = (command: "start" | "stop" | "restart") => async () => {
        const messages = {
            "start": "Container started",
            "stop": "Container stopped",
            "restart": "Container restarted"
        };
        try {
            if (containerId) {
                const res = await guiApi.containers.containersCreate(containerId, command);
                if (res.error) throw new Error(res.error.error);

                if (command === "start" || command === "restart") {
                    // restart stream & reset follow state
                    setData([]);
                    setAutoFollow(true);
                    atBottomRef.current = true;
                    stream.start(`/api/containers/${containerId}/logs`);
                } else {
                    stream?.stop();
                }

                await listContainers();
                notification.success({ message: messages[command] });
            }
        } catch (e: any) {
            notification.error({
                message: `Failed to ${command} container`,
                description: e.message
            });
        }
    };

    const selectedContainer = containers.find((c) => c.value === containerId);

    // --- Scroll handler: track whether user is at bottom ---
    useEffect(() => {
        const el = containerRef.current;
        if (!el) return;

        const onScroll = () => {
            const distance = el.scrollHeight - (el.scrollTop + el.clientHeight);
            const isAtBottom = distance <= THRESHOLD_PX;
            atBottomRef.current = isAtBottom;
            if (!isAtBottom) setAutoFollow(false);
        };

        el.addEventListener("scroll", onScroll, { passive: true });
        return () => el.removeEventListener("scroll", onScroll);
    }, []);

    // --- Handle container resize (line wraps, font size, viewport) while following ---
    useEffect(() => {
        const el = containerRef.current;
        if (!el) return;

        const ro = new ResizeObserver(() => {
            if (autoFollow) {
                requestAnimationFrame(() => {
                    el.scrollTo({ top: el.scrollHeight });
                });
            }
        });

        ro.observe(el);
        return () => ro.disconnect();
    }, [autoFollow]);

    // --- After new logs render: scroll if following or currently at bottom ---
    useLayoutEffect(() => {
        const el = containerRef.current;
        if (!el) return;

        if (autoFollow || atBottomRef.current) {
            // Use direct scroll for accuracy (fast, no flicker)
            el.scrollTo({ top: el.scrollHeight });
            // Alternative if you prefer smooth behavior:
            // bottomRef.current?.scrollIntoView({ block: "end", behavior: "auto" });
        }
    }, [data, autoFollow]);

    const jumpToLatest = (smooth = true) => {
        const el = containerRef.current;
        if (!el) return;
        el.scrollTo({ top: el.scrollHeight, behavior: smooth ? "smooth" : "auto" });
    };

    return (
        <Row style={{ height: "100%" , display: "flex", gap: 0, alignItems: "center", flexWrap: "wrap" }}>
            <Col span={24} style={{ display: "flex", gap: 0, alignItems: "center", flexWrap: "wrap" }}>
                <Typography.Title level={2}>Container logs</Typography.Title>
            </Col>

            <Col span={24}>
                <MowerActions/>
            </Col>

            <Col span={24} style={{ marginBottom: 20, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
                <Select<string>
                    options={containers}
                    value={containerId}
                    style={{ minWidth: 280 }}
                    onSelect={(value) => { setContainerId(value); }}
                />

                {selectedContainer && selectedContainer.status === "started" && (
                    <>
                        <AsyncButton onAsyncClick={commandContainer("restart")} style={{ marginRight: 10 }}>
                            Restart
                        </AsyncButton>
                        <AsyncButton
                            disabled={selectedContainer.labels.app == "gui"}
                            onAsyncClick={commandContainer("stop")}
                        >
                            Stop
                        </AsyncButton>
                    </>
                )}
                {selectedContainer && selectedContainer.status === "stopped" && (
                    <AsyncButton onAsyncClick={commandContainer("start")}>Start</AsyncButton>
                )}

                {/* Follow / Jump controls */}
                <Space size="small" style={{ marginLeft: "auto" }}>
                    <Button
                        type={autoFollow ? "primary" : "default"}
                        onClick={() => {
                            setAutoFollow(true);
                            atBottomRef.current = true;
                            jumpToLatest(true);
                        }}
                    >
                        {autoFollow ? "Following ✓" : "Follow latest"}
                    </Button>
                </Space>
            </Col>

            <Col span={24} >
                {/* Scrollable wrapper that we control */}
                <div
                    ref={containerRef}
                    style={{
                        // Make sure this element is the one that scrolls
                        height: "60vh",
                        overflow: "auto",
                        // Keep the look from StyledTerminal/Terminal
                        // You can tune these or rely entirely on StyledTerminal
                    }}
                >
                    <StyledTerminal>
                        <Terminal colorMode={ColorMode.Light}>
                            {data.map((line, index) => (
                                <TerminalOutput key={index}>
                                    <div dangerouslySetInnerHTML={{ __html: ansiHTML(line) }} />
                                </TerminalOutput>
                            ))}
                            <div ref={bottomRef} aria-hidden />
                        </Terminal>
                    </StyledTerminal>
                </div>
            </Col>
        </Row>
    );
};

export default LogsPage;
