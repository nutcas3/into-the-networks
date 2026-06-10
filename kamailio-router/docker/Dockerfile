FROM kamailio/kamailio:5.7.2-alpine

# Install additional modules
RUN apk add --no-cache \
    postgresql-client \
    curl \
    bash

# Copy configuration files
COPY cfg/kamailio.cfg /etc/kamailio/
COPY cfg/dispatcher.cfg /etc/kamailio/
COPY cfg/carrier.cfg /etc/kamailio/
COPY cfg/jsonrpc.cfg /etc/kamailio/

# Copy SQL schema
COPY sql/schema.sql /docker-entrypointdb.d/

# Set permissions
RUN chmod 644 /etc/kamailio/*.cfg

# Expose ports
EXPOSE 5060/udp 5060/tcp 5061/tcp 5062/tcp 5063/tcp 5064/tcp 5065/tcp 5066/tcp 5067/tcp 5068/tcp 5069/tcp 5070/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD kamcmd core.stats || exit 1

# Start Kamailio
CMD ["kamailio", "-D", "-E", "-e", "-P", "/run/kamailio/kamailio.pid"]
