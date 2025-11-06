import styled from "styled-components";

export const StyledTerminal = styled.div`
  div.react-terminal-wrapper {
    padding-top: 35px;
    background-color: #1a1e24;
  }

  div.react-terminal-wrapper > div.react-terminal-window-buttons {
    display: none;
  }

  div.react-terminal {
    height: auto;
  }

  div.react-terminal-light .react-terminal-line::before {
    display: initial;
    content: initial;
  }
`;