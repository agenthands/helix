// encodeMessage is a method of the Channel class that encodes a message string.
export class Channel {
  prefix: string;
  constructor(prefix: string) { this.prefix = prefix; }
  encodeMessage(msg: string): string {
    return this.prefix + ":" + msg;
  }
}

// sendBatch calls encodeMessage.
export function sendBatch(c: Channel, msgs: string[]): string[] {
  return msgs.map((m) => c.encodeMessage(m));
}
