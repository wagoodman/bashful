#/usr/bin/env bash
set -eux

NORMAL=$(tput sgr0)
RED=$(tput setaf 1)

pip install --user --upgrade --upgrade-strategy only-if-needed bridgy
if [[ ! -d ~/.bridgy ]]; then
    echo "Creating default config... ${BOLD}${RED}(you'll need an API key! Slack Alex Goodman to get one... only if you have your BI though)${NORMAL}"

    export PATH=~/Library/Python/2.7/bin:~/.local/bin:$PATH
    exportcmd='export PATH=~/Library/Python/2.7/bin:~/.local/bin:$PATH'
    sed -i.bak '/^export PATH=~\/Library\/Python\/2.7\/bin/d' ~/.bash_profile
    echo $exportcmd >> ~/.bash_profile

    # create a ~/.bridgy config dir
    bridgy ssh somethingthatwillnevermatch -d &>/dev/null

    # make secrets path for qa pulls
    mkdir -p $(pwd)/secrets/qa/

    # replace the config
    cat > ~/.bridgy/config.yml <<EOF
# what source should be used as an inventory source
inventory:
  source: newrelic
  update_at_start: true
  http_proxy: http://didit-proxy.uscis.dhs.gov:80
  https_proxy: http://didit-proxy.uscis.dhs.gov:80

# all newrelic inventory configuration
newrelic:
  account_number: 787121
  insights_query_api_key: API_KEY

# define ssh behavior and preferences
ssh:
  user: $(whoami)
  options: -C -o ForwardAgent=yes -o FingerprintHash=sha256 -o TCPKeepAlive=yes -o ServerAliveInterval=255 -o StrictHostKeyChecking=no
  no-tmux: true

# if you need to connect to aws hosts via a bastion, then
# provide all connectivity information here
bastion:
  user: $(whoami)
  address: gss-jumpbox.uscis.dhs.gov
  options: -C -o ServerAliveInterval=255 -o StrictHostKeyChecking=no -o FingerprintHash=sha256 -o TCPKeepAlive=yes -o ForwardAgent=yes -p 2222

sshfs:
  options: -o auto_cache,reconnect,no_readahead -C -o TCPKeepAlive=yes -o ServerAliveInterval=255 -o StrictHostKeyChecking=no -o sftp_server="/usr/bin/sudo /usr/lib/openssh/sftp-server"

ansible:
  become_user: root
  become_method: sudo

run:
  grab-secrets:
    - hosts: qa-public-site, qa-command-center, qa-profile, qa-digital-forms, qa-interactive-forms, qa-account-experience
      gather_facts: no
      tasks:
        - name: 'Discover app dir'
          find:
            paths: /webapps
            file_type: directory
            recurse: no
            patterns: "myuscis-*"
          register: app_dir
        - name: 'Get secrets.yml'
          fetch:
            src: "{{ item.path }}/config/secrets.yml"
            dest: $(pwd)/secrets/qa/{{ inventory_hostname }}.secrets.yml
            fail_on_missing: yes
            flat: yes
          with_items: "{{ app_dir.files }}"
        - name: 'Get production.rb'
          fetch:
            src: "{{ item.path }}/config/environments/production.rb"
            dest: $(pwd)/secrets/qa/{{ inventory_hostname }}.production.rb
            fail_on_missing: yes
            flat: yes
          with_items: "{{ app_dir.files }}"
        - name: 'Get database.yml'
          fetch:
            src: "{{ item.path }}/config/database.yml"
            dest: $(pwd)/secrets/qa/{{ inventory_hostname }}.database.yml
            fail_on_missing: no
            flat: yes
          with_items: "{{ app_dir.files }}"
        - name: 'Get myuscis-internal.crt'
          fetch:
            src: "/etc/nginx/myuscis-internal.crt"
            dest: $(pwd)/secrets/qa/myuscis-internal.crt
            fail_on_missing: no
            flat: yes
        - name: 'Get myuscis-internal.key'
          fetch:
            src: "/etc/nginx/myuscis-internal.key"
            dest: $(pwd)/secrets/qa/myuscis-internal.key
            fail_on_missing: no
            flat: yes
        - name: 'Get elis.crt'
          fetch:
            src: "/etc/nginx/elis.crt"
            dest: $(pwd)/secrets/qa/elis.crt
            fail_on_missing: no
            flat: yes
EOF
    fi
fi
